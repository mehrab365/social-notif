ALTER TABLE messages
  ADD COLUMN template_name     VARCHAR(128),
  ADD COLUMN template_language VARCHAR(16)  NOT NULL DEFAULT 'en_US',
  ADD COLUMN template_params   JSONB;
