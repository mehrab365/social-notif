ALTER TABLE messages
  ADD COLUMN shop_id UUID REFERENCES shops(id);
CREATE INDEX idx_messages_shop_id ON messages (shop_id);
