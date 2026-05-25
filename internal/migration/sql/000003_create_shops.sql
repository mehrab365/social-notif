CREATE TABLE shops (
    id                         UUID PRIMARY KEY,
    shop_domain                VARCHAR(255) NOT NULL UNIQUE,
    shopify_access_token       VARCHAR(512) NOT NULL DEFAULT '',
    whatsapp_access_token      VARCHAR(512) NOT NULL DEFAULT '',
    whatsapp_phone_number_id   VARCHAR(64)  NOT NULL DEFAULT '',
    whatsapp_template_name     VARCHAR(128) NOT NULL DEFAULT '',
    whatsapp_template_language VARCHAR(16)  NOT NULL DEFAULT 'en_US',
    created_at                 TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shops_shop_domain ON shops (shop_domain);
