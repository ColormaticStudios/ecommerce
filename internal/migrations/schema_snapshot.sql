TABLE brands
  COLUMN created_at
  COLUMN deleted_at
  COLUMN description
  COLUMN id
  COLUMN is_active
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
TABLE categories
  COLUMN created_at
  COLUMN deleted_at
  COLUMN depth
  COLUMN description
  COLUMN id
  COLUMN is_active
  COLUMN name
  COLUMN parent_id
  COLUMN path
  COLUMN slug
  COLUMN sort_order
  COLUMN updated_at
  INDEX idx_categories_deleted_at columns=deleted_at unique=false option=
  INDEX idx_categories_depth columns=depth unique=false option=
  INDEX idx_categories_is_active columns=is_active unique=false option=
  INDEX idx_categories_parent_id columns=parent_id unique=false option=
  INDEX idx_categories_path columns=path unique=false option=
  INDEX idx_categories_slug columns=slug unique=true option=
  INDEX idx_categories_sort_order columns=sort_order unique=false option=
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
TABLE cms_audit_events
  COLUMN action
  COLUMN actor
  COLUMN created_at
  COLUMN detail
  COLUMN entry_id
  COLUMN id
  COLUMN variant_id
  COLUMN version_id
  INDEX idx_cms_audit_entry_created columns=entry_id,created_at,id unique=false option=
  INDEX idx_cms_audit_events_action columns=action unique=false option=
  INDEX idx_cms_audit_events_created_at columns=created_at unique=false option=
  INDEX idx_cms_audit_events_entry_id columns=entry_id unique=false option=
  INDEX idx_cms_audit_events_variant_id columns=variant_id unique=false option=
  INDEX idx_cms_audit_events_version_id columns=version_id unique=false option=
TABLE cms_change_comments
  COLUMN actor
  COLUMN body
  COLUMN created_at
  COLUMN entry_id
  COLUMN id
  COLUMN resolved_at
  COLUMN resolved_by
  COLUMN variant_id
  INDEX idx_cms_change_comments_created_at columns=created_at unique=false option=
  INDEX idx_cms_change_comments_entry_id columns=entry_id unique=false option=
  INDEX idx_cms_change_comments_resolved_at columns=resolved_at unique=false option=
  INDEX idx_cms_change_comments_variant_id columns=variant_id unique=false option=
TABLE cms_content_variants
  COLUMN approved_by
  COLUMN created_at
  COLUMN deleted_at
  COLUMN draft_payload_json
  COLUMN entry_id
  COLUMN id
  COLUMN locale
  COLUMN market
  COLUMN published_at
  COLUMN published_payload_json
  COLUMN revision
  COLUMN status
  COLUMN submitted_by
  COLUMN updated_at
  INDEX idx_cms_content_variant_scope columns=entry_id,locale,market unique=false option=
  INDEX idx_cms_content_variant_scope_live columns=entry_id,locale,market unique=true option=
  INDEX idx_cms_content_variants_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_content_variants_published_at columns=published_at unique=false option=
  INDEX idx_cms_content_variants_status columns=status unique=false option=
TABLE cms_entries
  COLUMN created_at
  COLUMN current_version_id
  COLUMN deleted_at
  COLUMN entry_type
  COLUMN id
  COLUMN key
  COLUMN published_version_id
  COLUMN status
  COLUMN updated_at
  INDEX idx_cms_entries_current_version_id columns=current_version_id unique=false option=
  INDEX idx_cms_entries_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_entries_published_version_id columns=published_version_id unique=false option=
  INDEX idx_cms_entries_status columns=status unique=false option=
  INDEX idx_cms_entries_type_key columns=entry_type,key unique=false option=
  INDEX idx_cms_entries_type_key_live columns=entry_type,key unique=true option=
TABLE cms_entry_versions
  COLUMN change_summary
  COLUMN created_at
  COLUMN created_by
  COLUMN entry_id
  COLUMN id
  COLUMN payload_json
  COLUMN schema_version
  COLUMN version_number
  INDEX idx_cms_entry_versions_created_by columns=created_by unique=false option=
  INDEX idx_cms_versions_entry_number columns=entry_id,version_number unique=false option=
  INDEX idx_cms_versions_entry_number_unique columns=entry_id,version_number unique=true option=
TABLE cms_entry_workflows
  COLUMN approved_by
  COLUMN created_at
  COLUMN deleted_at
  COLUMN entry_id
  COLUMN id
  COLUMN status
  COLUMN submitted_by
  COLUMN updated_at
  COLUMN version_id
  INDEX idx_cms_entry_workflow_status columns=status,updated_at,id unique=false option=
  INDEX idx_cms_entry_workflows_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_entry_workflows_entry_id columns=entry_id unique=true option=
  INDEX idx_cms_entry_workflows_status columns=status unique=false option=
  INDEX idx_cms_entry_workflows_version_id columns=version_id unique=false option=
TABLE cms_experiment_variants
  COLUMN allocation
  COLUMN experiment_id
  COLUMN id
  COLUMN name
  COLUMN version_id
  INDEX idx_cms_experiment_variants_experiment_id columns=experiment_id unique=false option=
  INDEX idx_cms_experiment_variants_name columns=experiment_id,name unique=true option=
  INDEX idx_cms_experiment_variants_version_id columns=version_id unique=false option=
TABLE cms_experiments
  COLUMN created_at
  COLUMN deleted_at
  COLUMN ends_at
  COLUMN entry_id
  COLUMN id
  COLUMN name
  COLUMN starts_at
  COLUMN status
  COLUMN sticky_key
  COLUMN updated_at
  INDEX idx_cms_experiments_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_experiments_ends_at columns=ends_at unique=false option=
  INDEX idx_cms_experiments_entry_id columns=entry_id unique=false option=
  INDEX idx_cms_experiments_runtime columns=entry_id,status,starts_at,ends_at unique=false option=
  INDEX idx_cms_experiments_starts_at columns=starts_at unique=false option=
  INDEX idx_cms_experiments_status columns=status unique=false option=
TABLE cms_exposure_events
  COLUMN assignment_hash
  COLUMN content_version_id
  COLUMN correlation_id
  COLUMN created_at
  COLUMN entry_id
  COLUMN event_type
  COLUMN experiment_id
  COLUMN experiment_variant_id
  COLUMN id
  INDEX idx_cms_exposure_event_dedupe columns=correlation_id,event_type,content_version_id unique=true option=
  INDEX idx_cms_exposure_events_content_version_id columns=content_version_id unique=false option=
  INDEX idx_cms_exposure_events_correlation_id columns=correlation_id unique=false option=
  INDEX idx_cms_exposure_events_created_at columns=created_at unique=false option=
  INDEX idx_cms_exposure_events_entry_id columns=entry_id unique=false option=
  INDEX idx_cms_exposure_events_event_type columns=event_type unique=false option=
  INDEX idx_cms_exposure_events_experiment_id columns=experiment_id unique=false option=
  INDEX idx_cms_exposure_events_experiment_variant_id columns=experiment_variant_id unique=false option=
TABLE cms_global_regions
  COLUMN created_at
  COLUMN deleted_at
  COLUMN entry_id
  COLUMN id
  COLUMN key
  COLUMN region
  COLUMN title
  COLUMN updated_at
  INDEX idx_cms_global_regions_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_global_regions_entry_id columns=entry_id unique=true option=
  INDEX idx_cms_global_regions_key columns=key unique=true option=
  INDEX idx_cms_global_regions_key_live columns=key unique=true option=
  INDEX idx_cms_global_regions_region columns=region unique=false option=
TABLE cms_invalidation_events
  COLUMN attempts
  COLUMN created_at
  COLUMN entry_id
  COLUMN id
  COLUMN last_error
  COLUMN reason
  COLUMN sent_at
  COLUMN status
  COLUMN variant_id
  INDEX idx_cms_invalidation_events_created_at columns=created_at unique=false option=
  INDEX idx_cms_invalidation_events_entry_id columns=entry_id unique=false option=
  INDEX idx_cms_invalidation_events_status columns=status unique=false option=
  INDEX idx_cms_invalidation_events_variant_id columns=variant_id unique=false option=
  INDEX idx_cms_invalidation_pending columns=status,created_at,id unique=false option=
TABLE cms_locales
  COLUMN code
  COLUMN created_at
  COLUMN deleted_at
  COLUMN enabled
  COLUMN fallback_locale
  COLUMN id
  COLUMN is_default
  COLUMN name
  COLUMN updated_at
  INDEX idx_cms_locale_default columns=is_default unique=true option=
  INDEX idx_cms_locales_code columns=code unique=true option=
  INDEX idx_cms_locales_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_locales_enabled columns=enabled unique=false option=
  INDEX idx_cms_locales_is_default columns=is_default unique=false option=
TABLE cms_navigation_items
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN is_enabled
  COLUMN item_type
  COLUMN label
  COLUMN menu_id
  COLUMN parent_id
  COLUMN sort_order
  COLUMN target_ref
  COLUMN updated_at
  COLUMN url
  INDEX idx_cms_navigation_items_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_navigation_items_is_enabled columns=is_enabled unique=false option=
  INDEX idx_cms_navigation_items_menu_id columns=menu_id unique=false option=
  INDEX idx_cms_navigation_items_menu_order columns=menu_id,parent_id,sort_order,id unique=false option=
  INDEX idx_cms_navigation_items_parent_id columns=parent_id unique=false option=
  INDEX idx_cms_navigation_items_sort_order columns=sort_order unique=false option=
TABLE cms_navigation_menus
  COLUMN created_at
  COLUMN deleted_at
  COLUMN entry_id
  COLUMN id
  COLUMN key
  COLUMN location
  COLUMN title
  COLUMN updated_at
  INDEX idx_cms_navigation_menus_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_navigation_menus_entry_id columns=entry_id unique=true option=
  INDEX idx_cms_navigation_menus_key columns=key unique=true option=
  INDEX idx_cms_navigation_menus_key_live columns=key unique=true option=
  INDEX idx_cms_navigation_menus_location columns=location unique=false option=
TABLE cms_page_variants
  COLUMN approved_by
  COLUMN created_at
  COLUMN deleted_at
  COLUMN draft_payload_json
  COLUMN entry_id
  COLUMN id
  COLUMN locale
  COLUMN market
  COLUMN page_id
  COLUMN path
  COLUMN published_at
  COLUMN published_payload_json
  COLUMN revision
  COLUMN slug
  COLUMN status
  COLUMN submitted_by
  COLUMN title
  COLUMN updated_at
  INDEX idx_cms_page_variant_path_live columns=path,locale,market unique=true option=
  INDEX idx_cms_page_variant_scope columns=page_id,locale,market unique=false option=
  INDEX idx_cms_page_variant_scope_live columns=page_id,locale,market unique=true option=
  INDEX idx_cms_page_variants_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_page_variants_entry_id columns=entry_id unique=false option=
  INDEX idx_cms_page_variants_path columns=path unique=false option=
  INDEX idx_cms_page_variants_published_at columns=published_at unique=false option=
  INDEX idx_cms_page_variants_status columns=status unique=false option=
TABLE cms_pages
  COLUMN created_at
  COLUMN deleted_at
  COLUMN entry_id
  COLUMN id
  COLUMN is_homepage
  COLUMN path
  COLUMN seo_metadata_id
  COLUMN slug
  COLUMN template_key
  COLUMN title
  COLUMN updated_at
  COLUMN visibility
  INDEX idx_cms_pages_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_pages_entry_id columns=entry_id unique=true option=
  INDEX idx_cms_pages_homepage_live columns=is_homepage unique=true option=
  INDEX idx_cms_pages_is_homepage columns=is_homepage unique=false option=
  INDEX idx_cms_pages_path columns=path unique=false option=
  INDEX idx_cms_pages_path_live columns=path unique=true option=
  INDEX idx_cms_pages_seo_metadata_id columns=seo_metadata_id unique=false option=
  INDEX idx_cms_pages_slug columns=slug unique=false option=
  INDEX idx_cms_pages_visibility columns=visibility unique=false option=
TABLE cms_publications
  COLUMN entry_id
  COLUMN id
  COLUMN notes
  COLUMN published_at
  COLUMN published_by
  COLUMN rollback_from_publication_id
  COLUMN version_id
  INDEX idx_cms_publications_entry_id columns=entry_id unique=false option=
  INDEX idx_cms_publications_entry_published columns=entry_id,published_at,id unique=false option=
  INDEX idx_cms_publications_published_at columns=published_at unique=false option=
  INDEX idx_cms_publications_published_by columns=published_by unique=false option=
  INDEX idx_cms_publications_rollback_from_publication_id columns=rollback_from_publication_id unique=false option=
  INDEX idx_cms_publications_version_id columns=version_id unique=false option=
TABLE cms_redirect_rules
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN is_enabled
  COLUMN match_type
  COLUMN priority
  COLUMN redirect_type
  COLUMN source_pattern
  COLUMN target_url
  COLUMN updated_at
  INDEX idx_cms_redirect_rules_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_redirect_rules_is_enabled columns=is_enabled unique=false option=
  INDEX idx_cms_redirect_rules_match columns=is_enabled,match_type,priority,id unique=false option=
  INDEX idx_cms_redirect_rules_match_type columns=match_type unique=false option=
  INDEX idx_cms_redirect_rules_priority columns=priority unique=false option=
  INDEX idx_cms_redirect_rules_source_live columns=source_pattern,match_type unique=true option=
  INDEX idx_cms_redirect_rules_source_pattern columns=source_pattern unique=false option=
TABLE cms_role_assignments
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN role
  COLUMN subject
  COLUMN updated_at
  INDEX idx_cms_role_assignments_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_role_assignments_role columns=role unique=false option=
  INDEX idx_cms_role_assignments_subject columns=subject unique=true option=
TABLE cms_schedules
  COLUMN created_at
  COLUMN deleted_at
  COLUMN entry_id
  COLUMN id
  COLUMN last_transition_at
  COLUMN publish_at
  COLUMN status
  COLUMN timezone
  COLUMN unpublish_at
  COLUMN updated_at
  COLUMN version_id
  INDEX idx_cms_schedules_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_schedules_due columns=status,publish_at,unpublish_at unique=false option=
  INDEX idx_cms_schedules_entry_id columns=entry_id unique=true option=
  INDEX idx_cms_schedules_publish_at columns=publish_at unique=false option=
  INDEX idx_cms_schedules_status columns=status unique=false option=
  INDEX idx_cms_schedules_unpublish_at columns=unpublish_at unique=false option=
  INDEX idx_cms_schedules_version_id columns=version_id unique=false option=
TABLE cms_settings
  COLUMN approval_required
  COLUMN id
  COLUMN invalidation_webhook_url
  COLUMN updated_at
TABLE cms_targeting_rules
  COLUMN created_at
  COLUMN deleted_at
  COLUMN entry_id
  COLUMN id
  COLUMN is_enabled
  COLUMN priority
  COLUMN rule_json
  COLUMN updated_at
  COLUMN version_id
  INDEX idx_cms_targeting_rules_deleted_at columns=deleted_at unique=false option=
  INDEX idx_cms_targeting_rules_entry_id columns=entry_id unique=false option=
  INDEX idx_cms_targeting_rules_entry_priority columns=entry_id,is_enabled,priority,id unique=false option=
  INDEX idx_cms_targeting_rules_is_enabled columns=is_enabled unique=false option=
  INDEX idx_cms_targeting_rules_priority columns=priority unique=false option=
  INDEX idx_cms_targeting_rules_version_id columns=version_id unique=false option=
TABLE discount_campaign_audits
  COLUMN actor
  COLUMN after_json
  COLUMN before_json
  COLUMN campaign_id
  COLUMN changed_at
  COLUMN created_at
  COLUMN deleted_at
  COLUMN event_type
  COLUMN id
  COLUMN source
  COLUMN summary
  COLUMN updated_at
  INDEX idx_discount_campaign_audits_campaign_changed columns=campaign_id,changed_at unique=false option=
  INDEX idx_discount_campaign_audits_campaign_id columns=campaign_id unique=false option=
  INDEX idx_discount_campaign_audits_changed_at columns=changed_at unique=false option=
  INDEX idx_discount_campaign_audits_deleted_at columns=deleted_at unique=false option=
  INDEX idx_discount_campaign_audits_event_type columns=event_type unique=false option=
TABLE discount_campaigns
  COLUMN channels_json
  COLUMN coupon_code
  COLUMN created_at
  COLUMN created_by
  COLUMN customer_segment
  COLUMN deleted_at
  COLUMN discount_mode
  COLUMN discount_value
  COLUMN ends_at
  COLUMN global_usage_cap
  COLUMN id
  COLUMN is_archived
  COLUMN is_exclusive
  COLUMN metadata_json
  COLUMN name
  COLUMN per_customer_usage_cap
  COLUMN priority
  COLUMN starts_at
  COLUMN status
  COLUMN timezone
  COLUMN type
  COLUMN updated_at
  COLUMN updated_by
  INDEX idx_discount_campaigns_active_window columns=type,status,is_archived,starts_at,ends_at unique=false option=
  INDEX idx_discount_campaigns_coupon_code columns=coupon_code unique=true option=
  INDEX idx_discount_campaigns_created_by columns=created_by unique=false option=
  INDEX idx_discount_campaigns_deleted_at columns=deleted_at unique=false option=
  INDEX idx_discount_campaigns_ends_at columns=ends_at unique=false option=
  INDEX idx_discount_campaigns_is_archived columns=is_archived unique=false option=
  INDEX idx_discount_campaigns_priority columns=priority unique=false option=
  INDEX idx_discount_campaigns_runtime_lookup columns=status,is_archived,starts_at,ends_at,priority,id unique=false option=
  INDEX idx_discount_campaigns_starts_at columns=starts_at unique=false option=
  INDEX idx_discount_campaigns_status columns=status unique=false option=
  INDEX idx_discount_campaigns_type columns=type unique=false option=
  INDEX idx_discount_campaigns_updated_by columns=updated_by unique=false option=
TABLE discount_levels
  COLUMN action_json
  COLUMN campaign_id
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN max_applications_per_order
  COLUMN name
  COLUMN priority
  COLUMN stack_policy
  COLUMN updated_at
  INDEX idx_discount_levels_campaign_id columns=campaign_id unique=false option=
  INDEX idx_discount_levels_deleted_at columns=deleted_at unique=false option=
TABLE discount_redemptions
  COLUMN applied_amount
  COLUMN applied_at
  COLUMN campaign_id
  COLUMN created_at
  COLUMN customer_id
  COLUMN deleted_at
  COLUMN evaluation_snapshot_hash
  COLUMN id
  COLUMN level_id
  COLUMN order_id
  COLUMN updated_at
  INDEX idx_discount_redemptions_applied_at columns=applied_at unique=false option=
  INDEX idx_discount_redemptions_campaign_customer columns=campaign_id,customer_id unique=false option=
  INDEX idx_discount_redemptions_campaign_order columns=campaign_id,order_id unique=true option=
  INDEX idx_discount_redemptions_deleted_at columns=deleted_at unique=false option=
  INDEX idx_discount_redemptions_level_id columns=level_id unique=false option=
  INDEX idx_discount_redemptions_order_id columns=order_id unique=false option=
TABLE discount_rules
  COLUMN action_json
  COLUMN campaign_id
  COLUMN condition_json
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN max_applications_per_order
  COLUMN stack_policy
  COLUMN updated_at
  INDEX idx_discount_rules_campaign_id columns=campaign_id unique=false option=
  INDEX idx_discount_rules_deleted_at columns=deleted_at unique=false option=
TABLE discount_schedules
  COLUMN campaign_id
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN last_run_at
  COLUMN next_run_at
  COLUMN r_rule
  COLUMN schedule_type
  COLUMN timezone
  COLUMN until_at
  COLUMN updated_at
  COLUMN window_end
  COLUMN window_start
  INDEX idx_discount_schedules_campaign_id columns=campaign_id unique=true option=
  INDEX idx_discount_schedules_deleted_at columns=deleted_at unique=false option=
  INDEX idx_discount_schedules_next_run columns=next_run_at,schedule_type unique=false option=
  INDEX idx_discount_schedules_next_run_at columns=next_run_at unique=false option=
  INDEX idx_discount_schedules_schedule_type columns=schedule_type unique=false option=
  INDEX idx_discount_schedules_until_at columns=until_at unique=false option=
  INDEX idx_discount_schedules_window_end columns=window_end unique=false option=
  INDEX idx_discount_schedules_window_start columns=window_start unique=false option=
TABLE discount_state_histories
  COLUMN actor
  COLUMN campaign_id
  COLUMN changed_at
  COLUMN created_at
  COLUMN deleted_at
  COLUMN from_status
  COLUMN id
  COLUMN reason
  COLUMN source
  COLUMN to_status
  COLUMN updated_at
  INDEX idx_discount_state_histories_campaign_id columns=campaign_id unique=false option=
  INDEX idx_discount_state_histories_changed_at columns=changed_at unique=false option=
  INDEX idx_discount_state_histories_deleted_at columns=deleted_at unique=false option=
  INDEX idx_discount_state_history_campaign_changed columns=campaign_id,changed_at unique=false option=
TABLE discount_targets
  COLUMN campaign_id
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN level_id
  COLUMN target_id
  COLUMN target_type
  COLUMN updated_at
  INDEX idx_discount_target columns=campaign_id,level_id,target_type,target_id unique=true option=
  INDEX idx_discount_targets_category_lookup columns=target_type,target_id,campaign_id,level_id unique=false option=
  INDEX idx_discount_targets_deleted_at columns=deleted_at unique=false option=
  INDEX idx_discount_targets_level_id columns=level_id unique=false option=
  INDEX idx_discount_targets_level_lookup columns=level_id,target_type,target_id unique=false option=
  INDEX idx_discount_targets_product_lookup columns=target_type,target_id,campaign_id unique=false option=
TABLE idempotency_keys
  COLUMN checkout_session_id
  COLUMN correlation_id
  COLUMN created_at
  COLUMN expires_at
  COLUMN id
  COLUMN key
  COLUMN payment_intent_id
  COLUMN request_hash
  COLUMN response_body
  COLUMN response_code
  COLUMN scope
  COLUMN status
  COLUMN updated_at
  INDEX idx_idempotency_keys_correlation_id columns=correlation_id unique=false option=
  INDEX idx_idempotency_keys_expires_at columns=expires_at unique=false option=
  INDEX idx_idempotency_keys_payment_intent_id columns=payment_intent_id unique=false option=
  INDEX idx_idempotency_scope_session_key columns=scope,key,checkout_session_id unique=true option=
TABLE inventory_adjustments
  COLUMN actor_id
  COLUMN actor_type
  COLUMN approved_at
  COLUMN approved_by_id
  COLUMN approved_by_type
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN inventory_item_id
  COLUMN notes
  COLUMN product_variant_id
  COLUMN quantity_delta
  COLUMN reason_code
  COLUMN updated_at
  INDEX idx_inventory_adjustments_deleted_at columns=deleted_at unique=false option=
  INDEX idx_inventory_adjustments_inventory_item_id columns=inventory_item_id unique=false option=
  INDEX idx_inventory_adjustments_product_variant_id columns=product_variant_id unique=false option=
  INDEX idx_inventory_adjustments_reason_code columns=reason_code unique=false option=
TABLE inventory_alerts
  COLUMN acked_at
  COLUMN acked_by_id
  COLUMN acked_by_type
  COLUMN alert_type
  COLUMN available
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN inventory_item_id
  COLUMN opened_at
  COLUMN product_variant_id
  COLUMN resolved_at
  COLUMN resolved_by_id
  COLUMN resolved_by_type
  COLUMN status
  COLUMN threshold
  COLUMN updated_at
  INDEX idx_inventory_alerts_alert_type columns=alert_type unique=false option=
  INDEX idx_inventory_alerts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_inventory_alerts_inventory_item_id columns=inventory_item_id unique=false option=
  INDEX idx_inventory_alerts_opened_at columns=opened_at unique=false option=
  INDEX idx_inventory_alerts_product_variant_id columns=product_variant_id unique=false option=
  INDEX idx_inventory_alerts_status columns=status unique=false option=
TABLE inventory_items
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN product_variant_id
  COLUMN updated_at
  INDEX idx_inventory_items_deleted_at columns=deleted_at unique=false option=
  INDEX idx_inventory_items_product_variant_id columns=product_variant_id unique=true option=
TABLE inventory_levels
  COLUMN available
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN inventory_item_id
  COLUMN on_hand
  COLUMN reserved
  COLUMN updated_at
  INDEX idx_inventory_levels_deleted_at columns=deleted_at unique=false option=
  INDEX idx_inventory_levels_inventory_item_id columns=inventory_item_id unique=true option=
TABLE inventory_movements
  COLUMN actor_id
  COLUMN actor_type
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN inventory_item_id
  COLUMN movement_type
  COLUMN quantity_delta
  COLUMN reason_code
  COLUMN reference_id
  COLUMN reference_type
  COLUMN updated_at
  INDEX idx_inventory_movements_deleted_at columns=deleted_at unique=false option=
  INDEX idx_inventory_movements_inventory_item_id columns=inventory_item_id unique=false option=
  INDEX idx_inventory_movements_movement_type columns=movement_type unique=false option=
  INDEX idx_inventory_movements_reference_id columns=reference_id unique=false option=
  INDEX idx_inventory_movements_reference_type columns=reference_type unique=false option=
TABLE inventory_receipt_items
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN inventory_receipt_id
  COLUMN product_variant_id
  COLUMN purchase_order_item_id
  COLUMN quantity_received
  COLUMN updated_at
  INDEX idx_inventory_receipt_items_deleted_at columns=deleted_at unique=false option=
  INDEX idx_inventory_receipt_items_inventory_receipt_id columns=inventory_receipt_id unique=false option=
  INDEX idx_inventory_receipt_items_product_variant_id columns=product_variant_id unique=false option=
  INDEX idx_inventory_receipt_items_purchase_order_item_id columns=purchase_order_item_id unique=false option=
TABLE inventory_receipts
  COLUMN actor_id
  COLUMN actor_type
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN notes
  COLUMN purchase_order_id
  COLUMN received_at
  COLUMN updated_at
  INDEX idx_inventory_receipts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_inventory_receipts_purchase_order_id columns=purchase_order_id unique=false option=
  INDEX idx_inventory_receipts_received_at columns=received_at unique=false option=
TABLE inventory_reservations
  COLUMN checkout_session_id
  COLUMN consumed_at
  COLUMN created_at
  COLUMN deleted_at
  COLUMN expired_at
  COLUMN expires_at
  COLUMN id
  COLUMN idempotency_key
  COLUMN inventory_item_id
  COLUMN order_id
  COLUMN owner_id
  COLUMN owner_type
  COLUMN product_variant_id
  COLUMN quantity
  COLUMN released_at
  COLUMN status
  COLUMN updated_at
  INDEX idx_inventory_reservations_checkout_session_id columns=checkout_session_id unique=false option=
  INDEX idx_inventory_reservations_deleted_at columns=deleted_at unique=false option=
  INDEX idx_inventory_reservations_expires_at columns=expires_at unique=false option=
  INDEX idx_inventory_reservations_idempotency_key columns=idempotency_key unique=true option=
  INDEX idx_inventory_reservations_inventory_item_id columns=inventory_item_id unique=false option=
  INDEX idx_inventory_reservations_order_id columns=order_id unique=false option=
  INDEX idx_inventory_reservations_owner_id columns=owner_id unique=false option=
  INDEX idx_inventory_reservations_owner_type columns=owner_type unique=false option=
  INDEX idx_inventory_reservations_product_variant_id columns=product_variant_id unique=false option=
  INDEX idx_inventory_reservations_status columns=status unique=false option=
TABLE inventory_thresholds
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN low_stock_quantity
  COLUMN product_variant_id
  COLUMN updated_at
  INDEX idx_inventory_thresholds_deleted_at columns=deleted_at unique=false option=
  INDEX idx_inventory_thresholds_product_variant_id columns=product_variant_id unique=true option=
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
TABLE order_checkout_snapshot_items
  COLUMN created_at
  COLUMN id
  COLUMN price
  COLUMN product_variant_id
  COLUMN quantity
  COLUMN snapshot_id
  COLUMN updated_at
  COLUMN variant_sku
  COLUMN variant_title
  INDEX idx_order_checkout_snapshot_items_snapshot_id columns=snapshot_id unique=false option=
TABLE order_checkout_snapshots
  COLUMN authorized_at
  COLUMN checkout_session_id
  COLUMN created_at
  COLUMN currency
  COLUMN expires_at
  COLUMN id
  COLUMN order_id
  COLUMN payment_data_json
  COLUMN payment_method_display
  COLUMN payment_provider_id
  COLUMN shipping_address_pretty
  COLUMN shipping_amount
  COLUMN shipping_data_json
  COLUMN shipping_provider_id
  COLUMN subtotal
  COLUMN tax_amount
  COLUMN tax_data_json
  COLUMN tax_provider_id
  COLUMN total
  COLUMN updated_at
  INDEX idx_order_checkout_snapshots_checkout_session_id columns=checkout_session_id unique=false option=
  INDEX idx_order_checkout_snapshots_expires_at columns=expires_at unique=false option=
  INDEX idx_order_checkout_snapshots_order_id columns=order_id unique=false option=
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
TABLE order_status_histories
  COLUMN actor
  COLUMN correlation_id
  COLUMN created_at
  COLUMN from_status
  COLUMN id
  COLUMN order_id
  COLUMN reason
  COLUMN source
  COLUMN to_status
  INDEX idx_order_status_histories_correlation_id columns=correlation_id unique=false option=
  INDEX idx_order_status_histories_order_id columns=order_id unique=false option=
TABLE order_tax_lines
  COLUMN created_at
  COLUMN finalized_at
  COLUMN id
  COLUMN inclusive
  COLUMN jurisdiction
  COLUMN line_type
  COLUMN order_id
  COLUMN product_variant_id
  COLUMN quantity
  COLUMN snapshot_id
  COLUMN snapshot_item_id
  COLUMN tax_amount
  COLUMN tax_code
  COLUMN tax_name
  COLUMN tax_provider_id
  COLUMN tax_rate_basis_points
  COLUMN taxable_amount
  COLUMN updated_at
  INDEX idx_order_tax_lines_finalized_at columns=finalized_at unique=false option=
  INDEX idx_order_tax_lines_line_type columns=line_type unique=false option=
  INDEX idx_order_tax_lines_order_id columns=order_id unique=false option=
  INDEX idx_order_tax_lines_snapshot_id columns=snapshot_id unique=false option=
  INDEX idx_order_tax_lines_snapshot_item_id columns=snapshot_item_id unique=false option=
  INDEX idx_order_tax_lines_tax_provider_id columns=tax_provider_id unique=false option=
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
TABLE payment_intents
  COLUMN authorized_amount
  COLUMN captured_amount
  COLUMN created_at
  COLUMN currency
  COLUMN id
  COLUMN order_id
  COLUMN provider
  COLUMN snapshot_id
  COLUMN status
  COLUMN updated_at
  COLUMN version
  INDEX idx_payment_intents_order_id columns=order_id unique=false option=
  INDEX idx_payment_intents_snapshot_id columns=snapshot_id unique=false option=
  INDEX idx_payment_intents_status columns=status unique=false option=
TABLE payment_transactions
  COLUMN amount
  COLUMN created_at
  COLUMN id
  COLUMN idempotency_key
  COLUMN operation
  COLUMN payment_intent_id
  COLUMN provider_txn_id
  COLUMN raw_response_redacted
  COLUMN status
  COLUMN updated_at
  INDEX idx_payment_transactions_provider_txn_id columns=provider_txn_id unique=false option=
  INDEX idx_payment_transactions_status columns=status unique=false option=
  INDEX idx_payment_txn_intent_operation_key columns=payment_intent_id,operation,idempotency_key unique=true option=
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
  COLUMN enum_values
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
TABLE product_categories
  COLUMN category_id
  COLUMN product_id
  INDEX idx_product_categories_category_product columns=category_id,product_id unique=false option=
  INDEX idx_product_categories_product_category columns=product_id,category_id unique=true option=
TABLE product_category_drafts
  COLUMN category_id
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN position
  COLUMN product_draft_id
  COLUMN updated_at
  INDEX idx_product_category_drafts_category_id columns=category_id unique=false option=
  INDEX idx_product_category_drafts_deleted_at columns=deleted_at unique=false option=
  INDEX idx_product_category_drafts_product_draft_id columns=product_draft_id unique=false option=
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
TABLE promotion_templates
  COLUMN created_at
  COLUMN deleted_at
  COLUMN description
  COLUMN id
  COLUMN is_active
  COLUMN name
  COLUMN template_json
  COLUMN updated_at
  INDEX idx_promotion_templates_deleted_at columns=deleted_at unique=false option=
  INDEX idx_promotion_templates_is_active columns=is_active unique=false option=
TABLE provider_call_audits
  COLUMN correlation_id
  COLUMN created_at
  COLUMN environment
  COLUMN error_message
  COLUMN id
  COLUMN idempotency_key
  COLUMN latency_ms
  COLUMN operation
  COLUMN provider_id
  COLUMN provider_type
  COLUMN request_payload_redacted
  COLUMN response_payload_redacted
  COLUMN status
  COLUMN updated_at
  INDEX idx_provider_call_audits_correlation_id columns=correlation_id unique=false option=
  INDEX idx_provider_call_audits_environment columns=environment unique=false option=
  INDEX idx_provider_call_audits_idempotency_key columns=idempotency_key unique=false option=
  INDEX idx_provider_call_audits_operation columns=operation unique=false option=
  INDEX idx_provider_call_audits_provider_id columns=provider_id unique=false option=
  INDEX idx_provider_call_audits_provider_type columns=provider_type unique=false option=
  INDEX idx_provider_call_audits_status columns=status unique=false option=
TABLE provider_credentials
  COLUMN created_at
  COLUMN environment
  COLUMN id
  COLUMN key_version
  COLUMN label
  COLUMN last_rotated_at
  COLUMN metadata_json
  COLUMN provider_id
  COLUMN provider_type
  COLUMN secret_envelope_json
  COLUMN updated_at
  INDEX idx_provider_credentials_key_version columns=key_version unique=false option=
  INDEX idx_provider_credentials_last_rotated_at columns=last_rotated_at unique=false option=
  INDEX idx_provider_credentials_scope columns=provider_type,provider_id,environment unique=true option=
TABLE provider_operation_attempts
  COLUMN attempt_number
  COLUMN created_at
  COLUMN error_message
  COLUMN finished_at
  COLUMN id
  COLUMN operation_key
  COLUMN outcome
  COLUMN phase
  COLUMN provider_operation_id
  COLUMN provider_outcome
  COLUMN provider_reference
  COLUMN request_payload_redacted
  COLUMN response_payload_redacted
  COLUMN result_json
  COLUMN retryable
  COLUMN started_at
  COLUMN updated_at
  INDEX idx_provider_operation_attempt_number columns=provider_operation_id,attempt_number unique=true option=
  INDEX idx_provider_operation_attempts_operation_key columns=operation_key unique=false option=
  INDEX idx_provider_operation_attempts_outcome columns=outcome unique=false option=
  INDEX idx_provider_operation_attempts_phase columns=phase unique=false option=
  INDEX idx_provider_operation_attempts_provider_operation_id columns=provider_operation_id unique=false option=
  INDEX idx_provider_operation_attempts_provider_outcome columns=provider_outcome unique=false option=
  INDEX idx_provider_operation_attempts_provider_reference columns=provider_reference unique=false option=
TABLE provider_operations
  COLUMN completed_at
  COLUMN correlation_id
  COLUMN created_at
  COLUMN entity_id
  COLUMN entity_type
  COLUMN environment
  COLUMN id
  COLUMN idempotency_key
  COLUMN last_error
  COLUMN lease_expires_at
  COLUMN lease_owner
  COLUMN metadata_json
  COLUMN next_attempt_at
  COLUMN operation
  COLUMN operation_key
  COLUMN parent_operation_id
  COLUMN provider_id
  COLUMN provider_outcome
  COLUMN provider_reference
  COLUMN provider_type
  COLUMN request_fingerprint
  COLUMN request_json
  COLUMN result_json
  COLUMN status
  COLUMN updated_at
  COLUMN version
  INDEX idx_provider_operations_completed_at columns=completed_at unique=false option=
  INDEX idx_provider_operations_correlation_id columns=correlation_id unique=false option=
  INDEX idx_provider_operations_entity columns=entity_type,entity_id unique=false option=
  INDEX idx_provider_operations_idempotency columns=provider_type,provider_id,environment,operation,idempotency_key unique=true option=
  INDEX idx_provider_operations_lease_expires_at columns=lease_expires_at unique=false option=
  INDEX idx_provider_operations_lease_owner columns=lease_owner unique=false option=
  INDEX idx_provider_operations_next_attempt_at columns=next_attempt_at unique=false option=
  INDEX idx_provider_operations_operation_key columns=operation_key unique=true option=
  INDEX idx_provider_operations_parent_operation_id columns=parent_operation_id unique=false option=
  INDEX idx_provider_operations_provider_outcome columns=provider_outcome unique=false option=
  INDEX idx_provider_operations_provider_reference columns=provider_reference unique=false option=
  INDEX idx_provider_operations_scope_status columns=provider_type,provider_id,environment,operation,status unique=false option=
  INDEX idx_provider_operations_status columns=status unique=false option=
TABLE provider_reconciliation_cases
  COLUMN attempt_id
  COLUMN case_type
  COLUMN created_at
  COLUMN details_json
  COLUMN id
  COLUMN next_attempt_at
  COLUMN open_key
  COLUMN opened_at
  COLUMN operation_key
  COLUMN outcome
  COLUMN provider_operation_id
  COLUMN provider_outcome
  COLUMN reason
  COLUMN resolution_json
  COLUMN resolved_at
  COLUMN status
  COLUMN updated_at
  INDEX idx_provider_reconciliation_cases_attempt_id columns=attempt_id unique=false option=
  INDEX idx_provider_reconciliation_cases_case_type columns=case_type unique=false option=
  INDEX idx_provider_reconciliation_cases_next_attempt_at columns=next_attempt_at unique=false option=
  INDEX idx_provider_reconciliation_cases_open columns=provider_operation_id,open_key unique=true option=
  INDEX idx_provider_reconciliation_cases_operation_key columns=operation_key unique=false option=
  INDEX idx_provider_reconciliation_cases_outcome columns=outcome unique=false option=
  INDEX idx_provider_reconciliation_cases_provider_operation_id columns=provider_operation_id unique=false option=
  INDEX idx_provider_reconciliation_cases_provider_outcome columns=provider_outcome unique=false option=
  INDEX idx_provider_reconciliation_cases_resolved_at columns=resolved_at unique=false option=
  INDEX idx_provider_reconciliation_cases_status columns=status unique=false option=
TABLE provider_reconciliation_drifts
  COLUMN actual_value
  COLUMN created_at
  COLUMN entity_id
  COLUMN entity_type
  COLUMN expected_value
  COLUMN field_name
  COLUMN id
  COLUMN message
  COLUMN provider_reference
  COLUMN run_id
  COLUMN severity
  COLUMN updated_at
  INDEX idx_provider_reconciliation_drifts_entity_id columns=entity_id unique=false option=
  INDEX idx_provider_reconciliation_drifts_entity_type columns=entity_type unique=false option=
  INDEX idx_provider_reconciliation_drifts_provider_reference columns=provider_reference unique=false option=
  INDEX idx_provider_reconciliation_drifts_run_id columns=run_id unique=false option=
  INDEX idx_provider_reconciliation_drifts_severity columns=severity unique=false option=
TABLE provider_reconciliation_runs
  COLUMN checked_count
  COLUMN created_at
  COLUMN drift_count
  COLUMN environment
  COLUMN error_count
  COLUMN finished_at
  COLUMN id
  COLUMN provider_id
  COLUMN provider_type
  COLUMN started_at
  COLUMN status
  COLUMN summary_json
  COLUMN trigger
  COLUMN updated_at
  INDEX idx_provider_reconciliation_runs_environment columns=environment unique=false option=
  INDEX idx_provider_reconciliation_runs_finished_at columns=finished_at unique=false option=
  INDEX idx_provider_reconciliation_runs_provider_id columns=provider_id unique=false option=
  INDEX idx_provider_reconciliation_runs_provider_type columns=provider_type unique=false option=
  INDEX idx_provider_reconciliation_runs_status columns=status unique=false option=
  INDEX idx_provider_reconciliation_runs_trigger columns=trigger unique=false option=
TABLE purchase_order_items
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN product_variant_id
  COLUMN purchase_order_id
  COLUMN quantity_ordered
  COLUMN quantity_received
  COLUMN unit_cost
  COLUMN updated_at
  INDEX idx_purchase_order_items_deleted_at columns=deleted_at unique=false option=
  INDEX idx_purchase_order_items_product_variant_id columns=product_variant_id unique=false option=
  INDEX idx_purchase_order_items_purchase_order_id columns=purchase_order_id unique=false option=
TABLE purchase_orders
  COLUMN cancelled_at
  COLUMN created_at
  COLUMN deleted_at
  COLUMN id
  COLUMN issued_at
  COLUMN notes
  COLUMN received_at
  COLUMN status
  COLUMN supplier_id
  COLUMN updated_at
  INDEX idx_purchase_orders_deleted_at columns=deleted_at unique=false option=
  INDEX idx_purchase_orders_status columns=status unique=false option=
  INDEX idx_purchase_orders_supplier_id columns=supplier_id unique=false option=
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
  COLUMN json_ld
  COLUMN no_index
  COLUMN og_description
  COLUMN og_image_media_id
  COLUMN og_title
  COLUMN robots
  COLUMN title
  COLUMN twitter_card
  COLUMN twitter_description
  COLUMN twitter_image_media_id
  COLUMN twitter_title
  COLUMN updated_at
  INDEX idx_seo_entity columns=entity_type,entity_id unique=true option=
  INDEX idx_seo_metadata_canonical_path columns=canonical_path unique=true option=
  INDEX idx_seo_metadata_deleted_at columns=deleted_at unique=false option=
TABLE shipment_packages
  COLUMN created_at
  COLUMN height_cm
  COLUMN id
  COLUMN length_cm
  COLUMN reference
  COLUMN shipment_id
  COLUMN updated_at
  COLUMN weight_grams
  COLUMN width_cm
  INDEX idx_shipment_packages_shipment_id columns=shipment_id unique=false option=
TABLE shipment_rates
  COLUMN amount
  COLUMN created_at
  COLUMN currency
  COLUMN expires_at
  COLUMN id
  COLUMN order_id
  COLUMN provider
  COLUMN provider_rate_id
  COLUMN selected
  COLUMN service_code
  COLUMN service_name
  COLUMN shipment_id
  COLUMN snapshot_id
  COLUMN updated_at
  INDEX idx_shipment_rates_order_id columns=order_id unique=false option=
  INDEX idx_shipment_rates_selected columns=selected unique=false option=
  INDEX idx_shipment_rates_shipment_id columns=shipment_id unique=false option=
  INDEX idx_shipment_rates_snapshot_provider_rate columns=snapshot_id,provider,provider_rate_id unique=true option=
TABLE shipments
  COLUMN amount
  COLUMN created_at
  COLUMN currency
  COLUMN delivered_at
  COLUMN finalized_at
  COLUMN id
  COLUMN label_url
  COLUMN order_id
  COLUMN provider
  COLUMN provider_shipment_id
  COLUMN purchased_at
  COLUMN service_code
  COLUMN service_name
  COLUMN shipment_rate_id
  COLUMN shipping_address_pretty
  COLUMN snapshot_id
  COLUMN status
  COLUMN tracking_number
  COLUMN tracking_url
  COLUMN updated_at
  INDEX idx_shipments_finalized_at columns=finalized_at unique=false option=
  INDEX idx_shipments_order_id columns=order_id unique=false option=
  INDEX idx_shipments_provider columns=provider unique=false option=
  INDEX idx_shipments_provider_shipment_id columns=provider_shipment_id unique=false option=
  INDEX idx_shipments_shipment_rate_id columns=shipment_rate_id unique=true option=
  INDEX idx_shipments_snapshot_id columns=snapshot_id unique=false option=
  INDEX idx_shipments_status columns=status unique=false option=
TABLE suppliers
  COLUMN created_at
  COLUMN deleted_at
  COLUMN email
  COLUMN id
  COLUMN name
  COLUMN notes
  COLUMN updated_at
  INDEX idx_suppliers_deleted_at columns=deleted_at unique=false option=
  INDEX idx_suppliers_name columns=name unique=true option=
TABLE tax_exports
  COLUMN contents
  COLUMN created_at
  COLUMN exported_at
  COLUMN filters_json
  COLUMN format
  COLUMN id
  COLUMN provider
  COLUMN row_count
  COLUMN updated_at
  INDEX idx_tax_exports_exported_at columns=exported_at unique=false option=
  INDEX idx_tax_exports_provider columns=provider unique=false option=
TABLE tax_nexus_configs
  COLUMN active
  COLUMN country
  COLUMN created_at
  COLUMN exemption_code
  COLUMN id
  COLUMN inclusive_pricing
  COLUMN provider
  COLUMN state
  COLUMN updated_at
  INDEX idx_tax_nexus_provider_region columns=provider,country,state unique=true option=
TABLE tracking_events
  COLUMN created_at
  COLUMN description
  COLUMN id
  COLUMN location
  COLUMN occurred_at
  COLUMN provider
  COLUMN provider_event_id
  COLUMN raw_payload
  COLUMN shipment_id
  COLUMN status
  COLUMN tracking_number
  COLUMN updated_at
  INDEX idx_tracking_events_occurred_at columns=occurred_at unique=false option=
  INDEX idx_tracking_events_shipment_provider_event columns=shipment_id,provider,provider_event_id unique=true option=
  INDEX idx_tracking_events_status columns=status unique=false option=
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
TABLE webhook_events
  COLUMN attempt_count
  COLUMN created_at
  COLUMN event_type
  COLUMN id
  COLUMN last_error
  COLUMN payload
  COLUMN processed_at
  COLUMN provider
  COLUMN provider_event_id
  COLUMN received_at
  COLUMN signature_valid
  COLUMN updated_at
  INDEX idx_webhook_events_event_type columns=event_type unique=false option=
  INDEX idx_webhook_events_processed_at columns=processed_at unique=false option=
  INDEX idx_webhook_events_provider_event columns=provider,provider_event_id unique=true option=
  INDEX idx_webhook_events_received_at columns=received_at unique=false option=
TABLE website_settings
  COLUMN allow_guest_checkout
  COLUMN coupon_codes_enabled
  COLUMN created_at
  COLUMN id
  COLUMN oidc_client_id
  COLUMN oidc_client_secret_envelope_json
  COLUMN oidc_client_secret_key_version
  COLUMN oidc_provider
  COLUMN oidc_redirect_uri
  COLUMN site_title
  COLUMN updated_at
