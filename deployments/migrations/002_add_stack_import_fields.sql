-- Add stack import tracking fields
-- This migration adds fields to track which stacks were imported vs deployed through Flotilla

ALTER TABLE stacks ADD COLUMN IF NOT EXISTS imported BOOLEAN DEFAULT false;
ALTER TABLE stacks ADD COLUMN IF NOT EXISTS env_vars_sensitive BOOLEAN DEFAULT false;
ALTER TABLE stacks ADD COLUMN IF NOT EXISTS managed_by_flotilla BOOLEAN DEFAULT true;

-- Create index for imported stacks for faster filtering
CREATE INDEX IF NOT EXISTS idx_stacks_imported ON stacks(imported);
CREATE INDEX IF NOT EXISTS idx_stacks_managed_by_flotilla ON stacks(managed_by_flotilla);

-- Comment on columns
COMMENT ON COLUMN stacks.imported IS 'Indicates if this stack was imported from an existing deployment';
COMMENT ON COLUMN stacks.env_vars_sensitive IS 'Flags imported environment variables as potentially containing sensitive data';
COMMENT ON COLUMN stacks.managed_by_flotilla IS 'Indicates if the stack is managed by Flotilla (true) or manually deployed (false)';

