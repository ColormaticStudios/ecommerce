TABLE brands
  COLUMN created_at
  COLUMN deleted_at
  COLUMN description
  COLUMN id
  COLUMN is_active
  COLUMN logo_media_id
  COLUMN name
  COLUMN slug
  COLUMN updated_at
  INDEX idx_brands_deleted_at columns=deleted_at unique=false option=
  INDEX idx_brands_is_active columns=is_active unique=false option=
  INDEX idx_brands_slug columns=slug unique=true option=
TABLE cart_items
  COLUMN cart_id
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN product_id
  COLUMN product_variant_id
  COLUMN quantity
  COLUMN updated_at
  INDEX idx_cart_items_deleted_at columns=deleted_at unique=false option=
TABLE carts
  COLUMN checkout_session_id
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN updated_at
  INDEX idx_carts_checkout_session_id columns=checkout_session_id unique=true option=
  INDEX idx_carts_deleted_at columns=deleted_at unique=false option=
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
TABLE checkout_sessions
  COLUMN created_at
  COLUMN deleted_at
  COLUMN expires_at
  COLUMN guest_email
  COLUMN id
  COLUMN last_seen_at
  COLUMN public_token
  COLUMN status
  COLUMN updated_at
  COLUMN user_id
  INDEX idx_checkout_sessions_deleted_at columns=deleted_at unique=false option=
  INDEX idx_checkout_sessions_public_token columns=public_token unique=true option=
TABLE idempotency_keys
  COLUMN checkout_session_id
  COLUMN created_at
  COLUMN expires_at
  COLUMN id
  COLUMN key
  COLUMN request_hash
  COLUMN response_body
  COLUMN response_code
  COLUMN scope
  COLUMN updated_at
  INDEX idx_idempotency_keys_expires_at columns=expires_at unique=false option=
  INDEX idx_idempotency_scope_session_key columns=scope,key,checkout_session_id unique=true option=
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
  COLUMN product_variant_id
  COLUMN quantity
  COLUMN updated_at
  COLUMN variant_sku
  COLUMN variant_title
  INDEX idx_order_items_deleted_at columns=deleted_at unique=false option=
TABLE orders
  COLUMN checkout_session_id
  COLUMN claimed_at
  COLUMN confirmation_token
  COLUMN created_at
  COLUMN deleted_at
  COLUMN guest_email
  COLUMN id
  COLUMN payment_method_display
  COLUMN shipping_address_pretty
  COLUMN status
  COLUMN total
  COLUMN updated_at
  COLUMN user_id
  INDEX idx_orders_checkout_session_id columns=checkout_session_id unique=false option=
  INDEX idx_orders_confirmation_token columns=confirmation_token unique=true option=
  INDEX idx_orders_deleted_at columns=deleted_at unique=false option=
TABLE product_attribute_value_drafts
  COLUMN boolean_value
  COLUMN created_at
  COLUMN deleted_at
  COLUMN enum_value
  COLUMN id
  COLUMN is_deleted
  COLUMN number_value
  COLUMN position
  COLUMN product_attribute_id
  COLUMN product_draft_id
  COLUMN text_value
  COLUMN updated_at
  INDEX idx_product_attribute_value_drafts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_attribute_value_drafts_product_attribute_id columns=product_attribute_id unique=false option=
  INDEX idx_product_attribute_value_drafts_product_draft_id columns=product_draft_id unique=false option=
TABLE product_attribute_values
  COLUMN boolean_value
  COLUMN created_at
  COLUMN deleted_at
  COLUMN enum_value
  COLUMN id
  COLUMN number_value
  COLUMN position
  COLUMN product_attribute_id
  COLUMN product_id
  COLUMN text_value
  COLUMN updated_at
  INDEX idx_product_attribute_values_boolean_lookup columns=product_attribute_id,boolean_value,product_id unique=false option=
  INDEX idx_product_attribute_values_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_attribute_values_enum_lookup columns=product_attribute_id,enum_value,product_id unique=false option=
  INDEX idx_product_attribute_values_number_lookup columns=product_attribute_id,number_value,product_id unique=false option=
  INDEX idx_product_attribute_values_product_attribute_id columns=product_attribute_id unique=false option=
  INDEX idx_product_attribute_values_product_attribute_unique columns=product_id,product_attribute_id unique=true option=
  INDEX idx_product_attribute_values_product_id columns=product_id unique=false option=
  INDEX idx_product_attribute_values_text_lookup columns=product_attribute_id,text_value,product_id unique=false option=
TABLE product_attributes
  COLUMN created_at
  COLUMN deleted_at
  COLUMN filterable
  COLUMN id
  COLUMN key
  COLUMN slug
  COLUMN sortable
  COLUMN type
  COLUMN updated_at
  INDEX idx_product_attributes_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_attributes_filterable columns=filterable unique=false option=
  INDEX idx_product_attributes_key columns=key unique=true option=
  INDEX idx_product_attributes_slug columns=slug unique=true option=
  INDEX idx_product_attributes_sortable columns=sortable unique=false option=
TABLE product_drafts
  COLUMN brand_id
  COLUMN created_at
  COLUMN default_variant_sku
  COLUMN deleted_at
  COLUMN description
  COLUMN id
  COLUMN images_json
  COLUMN name
  COLUMN price
  COLUMN product_id
  COLUMN seo_canonical_path
  COLUMN seo_description
  COLUMN seo_no_index
  COLUMN seo_og_image_media_id
  COLUMN seo_title
  COLUMN sku
  COLUMN stock
  COLUMN subtitle
  COLUMN updated_at
  COLUMN version
  INDEX idx_product_drafts_brand_id columns=brand_id unique=false option=
  INDEX idx_product_drafts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_drafts_product_id columns=product_id unique=true option=
TABLE product_option_drafts
  COLUMN created_at
  COLUMN deleted_at
  COLUMN display_type
  COLUMN id
  COLUMN is_deleted
  COLUMN name
  COLUMN position
  COLUMN product_draft_id
  COLUMN source_product_option_id
  COLUMN updated_at
  INDEX idx_product_option_drafts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_option_drafts_product_draft_id columns=product_draft_id unique=false option=
  INDEX idx_product_option_drafts_source_product_option_id columns=source_product_option_id unique=false option=
TABLE product_option_value_drafts
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN is_deleted
  COLUMN position
  COLUMN product_option_draft_id
  COLUMN source_product_option_value_id
  COLUMN updated_at
  COLUMN value
  INDEX idx_product_option_value_drafts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_option_value_drafts_product_option_draft_id columns=product_option_draft_id unique=false option=
  INDEX idx_product_option_value_drafts_source_product_option_value_id columns=source_product_option_value_id unique=false option=
TABLE product_option_values
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN position
  COLUMN product_option_id
  COLUMN updated_at
  COLUMN value
  INDEX idx_product_option_values_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_option_values_option_value_unique columns=product_option_id,value unique=true option=
  INDEX idx_product_option_values_product_option_id columns=product_option_id unique=false option=
TABLE product_options
  COLUMN created_at
  COLUMN deleted_at
  COLUMN display_type
  COLUMN id
  COLUMN name
  COLUMN position
  COLUMN product_id
  COLUMN updated_at
  INDEX idx_product_options_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_options_product_id columns=product_id unique=false option=
  INDEX idx_product_options_product_name_unique columns=product_id,name unique=true option=
TABLE product_related
  COLUMN product_id
  COLUMN related_id
TABLE product_related_drafts
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN position
  COLUMN product_draft_id
  COLUMN related_product_id
  COLUMN updated_at
  INDEX idx_product_related_drafts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_related_drafts_product_draft_id columns=product_draft_id unique=false option=
  INDEX idx_product_related_drafts_related_product_id columns=related_product_id unique=false option=
TABLE product_variant_drafts
  COLUMN compare_at_price
  COLUMN created_at
  COLUMN deleted_at
  COLUMN height_cm
  COLUMN id
  COLUMN is_deleted
  COLUMN is_published
  COLUMN length_cm
  COLUMN position
  COLUMN price
  COLUMN product_draft_id
  COLUMN sku
  COLUMN source_product_variant_id
  COLUMN stock
  COLUMN title
  COLUMN updated_at
  COLUMN weight_grams
  COLUMN width_cm
  INDEX idx_product_variant_drafts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_variant_drafts_product_draft_id columns=product_draft_id unique=false option=
  INDEX idx_product_variant_drafts_source_product_variant_id columns=source_product_variant_id unique=false option=
TABLE product_variant_option_value_drafts
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN option_name
  COLUMN option_value
  COLUMN position
  COLUMN product_option_value_draft_id
  COLUMN product_variant_draft_id
  COLUMN source_product_option_value_id
  COLUMN updated_at
  INDEX idx_product_variant_option_value_drafts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_variant_option_value_drafts_product_option_v0bd648a0 columns=product_option_value_draft_id unique=false option=
  INDEX idx_product_variant_option_value_drafts_product_variant_draft_id columns=product_variant_draft_id unique=false option=
  INDEX idx_product_variant_option_value_drafts_source_product_o54f6a8e3 columns=source_product_option_value_id unique=false option=
TABLE product_variant_option_values
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN product_option_value_id
  COLUMN product_variant_id
  COLUMN updated_at
  INDEX idx_product_variant_option_values_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_variant_option_values_product_option_value_id columns=product_option_value_id unique=false option=
  INDEX idx_product_variant_option_values_product_variant_id columns=product_variant_id unique=false option=
  INDEX idx_product_variant_option_values_variant_value_unique columns=product_variant_id,product_option_value_id unique=true option=
TABLE product_variants
  COLUMN compare_at_price
  COLUMN created_at
  COLUMN deleted_at
  COLUMN height_cm
  COLUMN id
  COLUMN is_published
  COLUMN length_cm
  COLUMN position
  COLUMN price
  COLUMN product_id
  COLUMN sku
  COLUMN stock
  COLUMN title
  COLUMN updated_at
  COLUMN weight_grams
  COLUMN width_cm
  INDEX idx_product_variants_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_variants_is_published columns=is_published unique=false option=
  INDEX idx_product_variants_product_id columns=product_id unique=false option=
  INDEX idx_product_variants_product_published_price columns=product_id,is_published,price unique=false option=
  INDEX idx_product_variants_product_published_stock columns=product_id,is_published,stock unique=false option=
  INDEX idx_product_variants_sku columns=sku unique=false option=
  INDEX idx_product_variants_sku_unique columns=sku unique=true option=
TABLE products
  COLUMN brand_id
  COLUMN created_at
  COLUMN default_variant_id
  COLUMN deleted_at
  COLUMN description
  COLUMN draft_updated_at
  COLUMN id
  COLUMN images
  COLUMN is_published
  COLUMN name
  COLUMN price
  COLUMN sku
  COLUMN stock
  COLUMN subtitle
  COLUMN updated_at
  INDEX idx_products_brand_id columns=brand_id unique=false option=
  INDEX idx_products_brand_published_created_at columns=brand_id,is_published,created_at unique=false option=
  INDEX idx_products_default_variant_id columns=default_variant_id unique=false option=
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
TABLE seo_metadata
  COLUMN canonical_path
  COLUMN created_at
  COLUMN deleted_at
  COLUMN description
  COLUMN entity_id
  COLUMN entity_type
  COLUMN id
  COLUMN no_index
  COLUMN og_image_media_id
  COLUMN title
  COLUMN updated_at
  INDEX idx_seo_entity columns=entity_type,entity_id unique=true option=
  INDEX idx_seo_metadata_canonical_path columns=canonical_path unique=true option=
  INDEX idx_seo_metadata_deleted_at columns=deleted_at unique=false option=
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
