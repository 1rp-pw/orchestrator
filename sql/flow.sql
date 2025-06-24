-- Policy Management System with Draft and Versioning Support
-- Final Version with All Updates Applied

-- Create extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Main policies table
CREATE TABLE flows (
                       flow_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                       base_flow_id UUID NOT NULL, -- Groups all versions/drafts of the same policy
                       name VARCHAR(255) NOT NULL,
                       nodes JSONB NOT NULL,
                       edges JSONB NOT NULL,
                       tests JSONB NOT NULL,
                       flow TEXT NOT NULL,
                       version VARCHAR(50), -- NULL for drafts, 'v1.0', 'v1.1', etc. for versions
                       description TEXT, -- Required for versions, optional for drafts
                       status VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'version')),
                       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                       updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,

    -- Ensure only one draft per base_policy_id
                       CONSTRAINT unique_draft_per_flow
                           EXCLUDE (base_flow_id WITH =)
                           WHERE (status = 'draft'),

    -- Ensure version is unique per base_policy_id for versions
                       CONSTRAINT unique_version_per_flow
                           UNIQUE (base_flow_id, version),

    -- Ensure drafts don't have versions and versions have versions
                       CONSTRAINT draft_version_rules CHECK (
                           (status = 'draft' AND version IS NULL) OR
                           (status = 'version' AND version IS NOT NULL)
                           ),

    -- Ensure versions have descriptions but drafts don't require them
                       CONSTRAINT version_description_rules CHECK (
                           (status = 'version' AND description IS NOT NULL AND description != '') OR
                           (status = 'draft')
                           )
);

-- Index for efficient queries
CREATE INDEX idx_flows_base_flow_id ON flows(base_flow_id);
CREATE INDEX idx_flows_status ON flows(status);
CREATE INDEX idx_flows_version ON flows(version) WHERE version IS NOT NULL;

-- Trigger to automatically update updated_at
CREATE TRIGGER update_flows_updated_at
    BEFORE UPDATE ON flows
    FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- Function to create a new policy (creates initial draft)
CREATE OR REPLACE FUNCTION create_flow(
    p_name VARCHAR(255),
    p_nodes JSONB,
    p_edges JSONB,
    p_tests JSONB,
    p_flow TEXT
) RETURNS UUID AS $$
DECLARE
    new_base_flow_id UUID;
    new_flow_id UUID;
BEGIN
    new_base_flow_id := uuid_generate_v4();

    INSERT INTO flows (base_flow_id, name, nodes, edges, tests, flow, status)
    VALUES (new_base_flow_id, p_name, p_nodes, p_edges, p_tests, p_flow, 'draft')
    RETURNING flow_id INTO new_flow_id;

    RETURN new_flow_id;
END;
$$ LANGUAGE plpgsql;

-- Function to publish a draft as a version (removes draft after publishing)
CREATE OR REPLACE FUNCTION publish_draft_flow_as_version(
    p_base_flow_id UUID,
    p_version VARCHAR(50),
    p_description TEXT
) RETURNS UUID AS $$
DECLARE
    draft_flow_record RECORD;
    new_flow_id UUID;
BEGIN
    -- Validate description is provided
    IF p_description IS NULL OR p_description = '' THEN
        RAISE EXCEPTION 'Description is required when publishing a version';
    END IF;

    -- Get the current draft
    SELECT * INTO draft_flow_record
    FROM flows
    WHERE base_flow_id = p_base_flow_id AND status = 'draft';

    IF NOT FOUND THEN
        RAISE EXCEPTION 'No draft found for base_flow_id: %', p_base_flow_id;
    END IF;

    -- Check if version already exists
    IF EXISTS (SELECT 1 FROM flows WHERE base_flow_id = p_base_flow_id AND version = p_version) THEN
        RAISE EXCEPTION 'Version % already exists for this flow', p_version;
    END IF;

    -- Create new version record
    INSERT INTO flows (base_flow_id, name, nodes, edges, tests, flow, version, description, status)
    VALUES (
               draft_flow_record.base_flow_id,
               draft_flow_record.name,
               draft_flow_record.nodes,
               draft_flow_record.edges,
               draft_flow_record.tests,
               draft_flow_record.flow,
               p_version,
               p_description,
               'version'
           )
    RETURNING flow_id INTO new_flow_id;

    -- Remove the draft after successful version creation
    DELETE FROM flows
    WHERE base_flow_id = p_base_flow_id AND status = 'draft';

    RETURN new_flow_id;
END;
$$ LANGUAGE plpgsql;

-- Function to create a draft from a version
CREATE OR REPLACE FUNCTION create_draft_flow_from_version(
    p_base_flow_id UUID,
    p_version VARCHAR(50) DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
    source_flow_record RECORD;
    new_flow_id UUID;
BEGIN
    -- Delete existing draft if it exists
    DELETE FROM flows
    WHERE base_flow_id = p_base_flow_id AND status = 'draft';

    -- Get source version (latest if not specified)
    IF p_version IS NULL THEN
        SELECT * INTO source_flow_record
        FROM flows
        WHERE base_flow_id = p_base_flow_id AND status = 'version'
        ORDER BY created_at DESC
        LIMIT 1;
    ELSE
        SELECT * INTO source_flow_record
        FROM flows
        WHERE base_flow_id = p_base_flow_id AND version = p_version AND status = 'version';
    END IF;

    IF NOT FOUND THEN
        IF p_version IS NULL THEN
            RAISE EXCEPTION 'No versions found for base_flow_id: %', p_base_flow_id;
        ELSE
            RAISE EXCEPTION 'Version % not found for base_flow_id: %', p_version, p_base_flow_id;
        END IF;
    END IF;

    -- Create new draft
    INSERT INTO flows (base_flow_id, name, nodes, edges, tests, flow, status)
    VALUES (
               source_flow_record.base_flow_id,
               source_flow_record.name,
               source_flow_record.nodes,
               source_flow_record.edges,
               source_flow_record.tests,
               source_flow_record.flow,
               'draft'
           )
    RETURNING flow_id INTO new_flow_id;

    RETURN new_flow_id;
END;
$$ LANGUAGE plpgsql;

-- Function to update a draft (name cannot be changed)
CREATE OR REPLACE FUNCTION update_draft_flow(
    p_base_flow_id UUID,
    p_nodes JSONB DEFAULT NULL,
    p_edges JSONB DEFAULT NULL,
    p_tests JSONB DEFAULT NULL,
    p_flow TEXT DEFAULT NULL,
    p_description TEXT DEFAULT NULL
) RETURNS BOOLEAN AS $$
BEGIN
    UPDATE flows
    SET
        nodes = COALESCE(p_nodes, nodes),
        edges = COALESCE(p_edges, edges),
        tests = COALESCE(p_tests, tests),
        flow = COALESCE(p_flow, flow),
        description = COALESCE(p_description, description)
    WHERE base_flow_id = p_base_flow_id AND status = 'draft';

    RETURN FOUND;
END;
$$ LANGUAGE plpgsql;

-- View to list all policies with their summary information
CREATE VIEW flow_summary AS
SELECT DISTINCT
    base_flow_id,
    FIRST_VALUE(name) OVER (PARTITION BY base_flow_id ORDER BY
        CASE WHEN status = 'draft' THEN 1 ELSE 2 END, created_at DESC) as current_name,
    COUNT(*) FILTER (WHERE status = 'version') OVER (PARTITION BY base_flow_id) as version_count,
    FIRST_VALUE(CASE WHEN status = 'draft' THEN flow_id END) OVER (
        PARTITION BY base_flow_id
        ORDER BY CASE WHEN status = 'draft' THEN created_at END DESC NULLS LAST
        ) as draft_id,
    MIN(created_at) OVER (PARTITION BY base_flow_id) as first_created_date,
    MAX(created_at) FILTER (WHERE status = 'version') OVER (PARTITION BY base_flow_id) as latest_version_date,
    MAX(updated_at) OVER (PARTITION BY base_flow_id) as latest_activity_date,
    BOOL_OR(status = 'draft') OVER (PARTITION BY base_flow_id) as has_draft
FROM flows;

-- Example usage and sample data
INSERT INTO flows (base_flow_id, name, nodes, edges, tests, flow, status) VALUES
    (uuid_generate_v4(), 'User Authentication Flow', '[
      {
        "id": "start-1",
        "type": "start",
        "label": "Start",
        "policyId": "",
        "policyName": ""
      },
      {
        "id": "return-1750197119240",
        "type": "return",
        "label": "Return True",
        "returnValue": true
      },
      {
        "id": "return-1750197119829",
        "type": "return",
        "label": "Return False",
        "returnValue": false
      }
    ]', '[
      {
        "id": "edge-start-1-return-1750197119240",
        "source": "start-1",
        "target": "return-1750197119240",
        "sourceHandle": "true",
        "label": "True",
        "style": {
          "stroke": "#22c55e",
          "strokeWidth": 2
        },
        "labelStyle": {
          "fill": "#22c55e",
          "fontWeight": 600
        }
      },
      {
        "id": "edge-start-1-return-1750197119829",
        "source": "start-1",
        "target": "return-1750197119829",
        "sourceHandle": "false",
        "label": "False",
        "style": {
          "stroke": "#ef4444",
          "strokeWidth": 2
        },
        "labelStyle": {
          "fill": "#ef4444",
          "fontWeight": 600
        }
      }
    ]', '[
      {
        "id": "default-1",
        "name": "Test 1",
        "data": "{\n  \"example\": \"data\",\n  \"value\": 123\n}",
        "expectedOutcome": true,
        "created": true,
        "createdAt": "2025-06-17T22:42:27.457Z"
      }
    ]', 'Users must authenticate with valid credentials before accessing protected resources', 'draft');

-- Sample queries and usage examples:

-- 1. Create a new policy
-- SELECT create_flow('Data Retention Flow', '{"retention_days": 90}', '{"tests": ["test_retention"]}', 'Data must be retained for 90 days then automatically purged');

-- 2. List all versions of a specific policy
-- SELECT policy_id, name, version, rule, description, status, created_at, updated_at
-- FROM policies
-- WHERE base_policy_id = 'your-base-policy-id'
-- ORDER BY
--     CASE WHEN status = 'draft' THEN 0 ELSE 1 END,
--     CASE WHEN version IS NULL THEN '' ELSE version END;

-- 3. Get policy summary
-- SELECT * FROM policy_summary ORDER BY current_name;

-- 4. Publish draft as version (removes draft)
-- SELECT publish_draft_as_version('your-base-policy-id', 'v1.0', 'Initial release with basic authentication rules');

-- 5. Create draft from version
-- SELECT create_draft_from_version('your-base-policy-id', 'v1.0');

-- 6. Update a draft (no name changes allowed)
-- SELECT update_draft('your-base-policy-id', '{"new": "data"}', '{"new": "tests"}', 'Updated rule text', 'Work in progress description');

-- 7. Example workflow:
-- Step 1: Create policy (creates draft)
-- SELECT create_policy('Example Policy', '{"setting": "value"}', '{"test": "case"}', 'Example rule text');

-- Step 2: Update draft as needed
-- SELECT update_draft('base-policy-uuid', '{"updated_setting": "new_value"}', NULL, 'Updated rule', 'Added new setting');

-- Step 3: Publish version (removes draft)
-- SELECT publish_draft_as_version('base-policy-uuid', 'v1.0', 'Initial production release');

-- Step 4: Create new draft from published version to continue development
-- SELECT create_draft_from_version('base-policy-uuid', 'v1.0');

-- Step 5: Make more changes and publish v1.1
-- SELECT update_draft('base-policy-uuid', '{"new_feature": true}', NULL, 'Added new feature', NULL);
-- SELECT publish_draft_as_version('base-policy-uuid', 'v1.1', 'Added new feature support');