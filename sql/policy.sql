-- Policy Management System with Draft and Versioning Support
-- Final Version with All Updates Applied

-- Create extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Main policies table
CREATE TABLE policies (
                          policy_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
                          base_policy_id UUID NOT NULL, -- Groups all versions/drafts of the same policy
                          name VARCHAR(255) NOT NULL,
                          data_model JSONB NOT NULL,
                          tests JSONB NOT NULL,
                          rule TEXT NOT NULL,
                          version VARCHAR(50), -- NULL for drafts, 'v1.0', 'v1.1', etc. for versions
                          description TEXT, -- Required for versions, optional for drafts
                          status VARCHAR(20) NOT NULL CHECK (status IN ('draft', 'version')),
                          created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                          updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,

    -- Ensure only one draft per base_policy_id
                          CONSTRAINT unique_draft_per_policy
                              EXCLUDE (base_policy_id WITH =)
                              WHERE (status = 'draft'),

    -- Ensure version is unique per base_policy_id for versions
                          CONSTRAINT unique_version_per_policy
                              UNIQUE (base_policy_id, version),

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
CREATE INDEX idx_policies_base_policy_id ON policies(base_policy_id);
CREATE INDEX idx_policies_status ON policies(status);
CREATE INDEX idx_policies_version ON policies(version) WHERE version IS NOT NULL;

-- Function to update the updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
    RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update updated_at
CREATE TRIGGER update_policies_updated_at
    BEFORE UPDATE ON policies
    FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- Function to create a new policy (creates initial draft)
CREATE OR REPLACE FUNCTION create_policy(
    p_name VARCHAR(255),
    p_data_model JSONB,
    p_tests JSONB,
    p_rule TEXT
) RETURNS UUID AS $$
DECLARE
    new_base_policy_id UUID;
    new_policy_id UUID;
BEGIN
    new_base_policy_id := uuid_generate_v4();

    INSERT INTO policies (base_policy_id, name, data_model, tests, rule, status)
    VALUES (new_base_policy_id, p_name, p_data_model, p_tests, p_rule, 'draft')
    RETURNING policy_id INTO new_policy_id;

    RETURN new_policy_id;
END;
$$ LANGUAGE plpgsql;

-- Function to publish a draft as a version (removes draft after publishing)
CREATE OR REPLACE FUNCTION publish_draft_as_version(
    p_base_policy_id UUID,
    p_version VARCHAR(50),
    p_description TEXT
) RETURNS UUID AS $$
DECLARE
    draft_record RECORD;
    new_policy_id UUID;
BEGIN
    -- Validate description is provided
    IF p_description IS NULL OR p_description = '' THEN
        RAISE EXCEPTION 'Description is required when publishing a version';
    END IF;

    -- Get the current draft
    SELECT * INTO draft_record
    FROM policies
    WHERE base_policy_id = p_base_policy_id AND status = 'draft';

    IF NOT FOUND THEN
        RAISE EXCEPTION 'No draft found for base_policy_id: %', p_base_policy_id;
    END IF;

    -- Check if version already exists
    IF EXISTS (SELECT 1 FROM policies WHERE base_policy_id = p_base_policy_id AND version = p_version) THEN
        RAISE EXCEPTION 'Version % already exists for this policy', p_version;
    END IF;

    -- Create new version record
    INSERT INTO policies (base_policy_id, name, data_model, tests, rule, version, description, status)
    VALUES (
               draft_record.base_policy_id,
               draft_record.name,
               draft_record.data_model,
               draft_record.tests,
               draft_record.rule,
               p_version,
               p_description,
               'version'
           )
    RETURNING policy_id INTO new_policy_id;

    -- Remove the draft after successful version creation
    DELETE FROM policies
    WHERE base_policy_id = p_base_policy_id AND status = 'draft';

    RETURN new_policy_id;
END;
$$ LANGUAGE plpgsql;

-- Function to create a draft from a version
CREATE OR REPLACE FUNCTION create_draft_from_version(
    p_base_policy_id UUID,
    p_version VARCHAR(50) DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
    source_record RECORD;
    new_policy_id UUID;
BEGIN
    -- Delete existing draft if it exists
    DELETE FROM policies
    WHERE base_policy_id = p_base_policy_id AND status = 'draft';

    -- Get source version (latest if not specified)
    IF p_version IS NULL THEN
        SELECT * INTO source_record
        FROM policies
        WHERE base_policy_id = p_base_policy_id AND status = 'version'
        ORDER BY created_at DESC
        LIMIT 1;
    ELSE
        SELECT * INTO source_record
        FROM policies
        WHERE base_policy_id = p_base_policy_id AND version = p_version AND status = 'version';
    END IF;

    IF NOT FOUND THEN
        IF p_version IS NULL THEN
            RAISE EXCEPTION 'No versions found for base_policy_id: %', p_base_policy_id;
        ELSE
            RAISE EXCEPTION 'Version % not found for base_policy_id: %', p_version, p_base_policy_id;
        END IF;
    END IF;

    -- Create new draft
    INSERT INTO policies (base_policy_id, name, data_model, tests, rule, status)
    VALUES (
               source_record.base_policy_id,
               source_record.name,
               source_record.data_model,
               source_record.tests,
               source_record.rule,
               'draft'
           )
    RETURNING policy_id INTO new_policy_id;

    RETURN new_policy_id;
END;
$$ LANGUAGE plpgsql;

-- Function to update a draft (name cannot be changed)
CREATE OR REPLACE FUNCTION update_draft(
    p_base_policy_id UUID,
    p_data_model JSONB DEFAULT NULL,
    p_tests JSONB DEFAULT NULL,
    p_rule TEXT DEFAULT NULL,
    p_description TEXT DEFAULT NULL
) RETURNS BOOLEAN AS $$
BEGIN
    UPDATE policies
    SET
        data_model = COALESCE(p_data_model, data_model),
        tests = COALESCE(p_tests, tests),
        rule = COALESCE(p_rule, rule),
        description = COALESCE(p_description, description)
    WHERE base_policy_id = p_base_policy_id AND status = 'draft';

    RETURN FOUND;
END;
$$ LANGUAGE plpgsql;

-- View to list all policies with their summary information
CREATE VIEW policy_summary AS
SELECT DISTINCT
    base_policy_id,
    FIRST_VALUE(name) OVER (PARTITION BY base_policy_id ORDER BY
        CASE WHEN status = 'draft' THEN 1 ELSE 2 END, created_at DESC) as current_name,
    COUNT(*) FILTER (WHERE status = 'version') OVER (PARTITION BY base_policy_id) as version_count,
    FIRST_VALUE(CASE WHEN status = 'draft' THEN policy_id END) OVER (
        PARTITION BY base_policy_id
        ORDER BY CASE WHEN status = 'draft' THEN created_at END DESC NULLS LAST
        ) as draft_id,
    MIN(created_at) OVER (PARTITION BY base_policy_id) as first_created_date,
    MAX(created_at) FILTER (WHERE status = 'version') OVER (PARTITION BY base_policy_id) as latest_version_date,
    MAX(updated_at) OVER (PARTITION BY base_policy_id) as latest_activity_date,
    BOOL_OR(status = 'draft') OVER (PARTITION BY base_policy_id) as has_draft
FROM policies;

-- Example usage and sample data
INSERT INTO policies (base_policy_id, name, data_model, tests, rule, status) VALUES
    (uuid_generate_v4(), 'User Authentication Policy', '{"rules": {"min_password_length": 8}}', '{"test_cases": []}', 'Users must authenticate with valid credentials before accessing protected resources', 'draft');

-- Sample queries and usage examples:

-- 1. Create a new policy
-- SELECT create_policy('Data Retention Policy', '{"retention_days": 90}', '{"tests": ["test_retention"]}', 'Data must be retained for 90 days then automatically purged');

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