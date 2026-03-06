TABLE cart_items
  COLUMN cart_id
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN product_id
  COLUMN quantity
  COLUMN updated_at
  INDEX idx_cart_items_deleted_at columns=deleted_at unique=false option=
TABLE carts
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN updated_at
  COLUMN user_id
  INDEX idx_carts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_carts_user_id columns=user_id unique=true option=
TABLE checkout_provider_settings
  COLUMN created_at
  COLUMN deleted_at
  COLUMN enabled
  COLUMN id
  COLUMN provider_id
  COLUMN provider_type
  COLUMN updated_at
  INDEX idx_checkout_provider_settings_deleted_at columns=deleted_at unique=false option=
  INDEX idx_checkout_provider_settings_type_id columns=provider_type,provider_id unique=true option=
TABLE media_objects
  COLUMN created_at
  COLUMN id
  COLUMN mime_type
  COLUMN original_path
  COLUMN size_bytes
  COLUMN status
  COLUMN updated_at
TABLE media_references
  COLUMN created_at
  COLUMN id
  COLUMN media_id
  COLUMN owner_id
  COLUMN owner_type
  COLUMN position
  COLUMN role
  INDEX idx_media_references_media_id columns=media_id unique=false option=
  INDEX idx_media_references_owner_id columns=owner_id unique=false option=
  INDEX idx_media_references_owner_type columns=owner_type unique=false option=
  INDEX idx_media_references_role columns=role unique=false option=
TABLE media_variants
  COLUMN created_at
  COLUMN height
  COLUMN id
  COLUMN label
  COLUMN media_id
  COLUMN mime_type
  COLUMN path
  COLUMN size_bytes
  COLUMN width
  INDEX idx_media_variants_media_id columns=media_id unique=false option=
TABLE order_items
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN order_id
  COLUMN price
  COLUMN product_id
  COLUMN quantity
  COLUMN updated_at
  INDEX idx_order_items_deleted_at columns=deleted_at unique=false option=
TABLE orders
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN payment_method_display
  COLUMN shipping_address_pretty
  COLUMN status
  COLUMN total
  COLUMN updated_at
  COLUMN user_id
  INDEX idx_orders_deleted_at columns=deleted_at unique=false option=
TABLE product_related
  COLUMN product_id
  COLUMN related_id
TABLE products
  COLUMN created_at
  COLUMN deleted_at
  COLUMN description
  COLUMN draft_data
  COLUMN draft_updated_at
  COLUMN id
  COLUMN images
  COLUMN is_published
  COLUMN name
  COLUMN price
  COLUMN sku
  COLUMN stock
  COLUMN updated_at
  INDEX idx_products_deleted_at columns=deleted_at unique=false option=
  INDEX idx_products_is_published columns=is_published unique=false option=
TABLE saved_addresses
  COLUMN city
  COLUMN country
  COLUMN created_at
  COLUMN deleted_at
  COLUMN full_name
  COLUMN id
  COLUMN is_default
  COLUMN label
  COLUMN line1
  COLUMN line2
  COLUMN phone
  COLUMN postal_code
  COLUMN state
  COLUMN updated_at
  COLUMN user_id
  INDEX idx_saved_addresses_deleted_at columns=deleted_at unique=false option=
  INDEX idx_saved_addresses_user_id columns=user_id unique=false option=
TABLE saved_payment_methods
  COLUMN brand
  COLUMN cardholder_name
  COLUMN created_at
  COLUMN deleted_at
  COLUMN exp_month
  COLUMN exp_year
  COLUMN id
  COLUMN is_default
  COLUMN last4
  COLUMN nickname
  COLUMN type
  COLUMN updated_at
  COLUMN user_id
  INDEX idx_saved_payment_methods_deleted_at columns=deleted_at unique=false option=
  INDEX idx_saved_payment_methods_user_id columns=user_id unique=false option=
TABLE schema_migrations
  COLUMN applied_at
  COLUMN checksum
  COLUMN duration_ms
  COLUMN execution_meta
  COLUMN name
  COLUMN version
TABLE storefront_settings
  COLUMN config_json
  COLUMN created_at
  COLUMN draft_config_json
  COLUMN draft_updated_at
  COLUMN id
  COLUMN published_updated
  COLUMN updated_at
TABLE users
  COLUMN created_at
  COLUMN currency
  COLUMN deleted_at
  COLUMN email
  COLUMN id
  COLUMN name
  COLUMN password_hash
  COLUMN profile_photo
  COLUMN role
  COLUMN subject
  COLUMN updated_at
  COLUMN username
  INDEX idx_users_deleted_at columns=deleted_at unique=false option=
  INDEX idx_users_email columns=email unique=true option=
  INDEX idx_users_subject columns=subject unique=true option=
  INDEX idx_users_username columns=username unique=true option=
