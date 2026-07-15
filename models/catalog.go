package models

type Brand struct {
	BaseModel
	Name        string  `json:"name" gorm:"not null"`
	Slug        string  `json:"slug" gorm:"uniqueIndex;not null"`
	Description *string `json:"description,omitempty"`
	IsActive    bool    `json:"is_active" gorm:"not null;default:true;index"`
}

type Category struct {
	BaseModel
	Name        string     `json:"name" gorm:"not null"`
	Slug        string     `json:"slug" gorm:"uniqueIndex;not null"`
	Description *string    `json:"description,omitempty"`
	IsActive    bool       `json:"is_active" gorm:"not null;default:true;index"`
	SortOrder   int        `json:"sort_order" gorm:"not null;default:0;index"`
	ParentID    *uint      `json:"parent_id,omitempty" gorm:"index"`
	Parent      *Category  `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children    []Category `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	Path        string     `json:"path" gorm:"not null;index"`
	Depth       int        `json:"depth" gorm:"not null;default:0;index"`
}

type ProductCategory struct {
	ProductID  uint `json:"product_id" gorm:"primaryKey;autoIncrement:false"`
	CategoryID uint `json:"category_id" gorm:"primaryKey;autoIncrement:false"`
}

type ProductOption struct {
	BaseModel
	ProductID   uint                 `json:"product_id" gorm:"not null;index"`
	Name        string               `json:"name" gorm:"not null"`
	Position    int                  `json:"position" gorm:"not null;default:1"`
	DisplayType string               `json:"display_type" gorm:"not null;default:select"`
	Values      []ProductOptionValue `json:"values,omitempty"`
}

type ProductOptionValue struct {
	BaseModel
	ProductOptionID uint   `json:"product_option_id" gorm:"not null;index"`
	Value           string `json:"value" gorm:"not null"`
	Position        int    `json:"position" gorm:"not null;default:1"`
}

type ProductVariant struct {
	BaseModel
	ProductID        uint                        `json:"product_id" gorm:"not null;index"`
	Product          Product                     `json:"-" gorm:"foreignKey:ProductID"`
	SKU              string                      `json:"sku" gorm:"not null;index"`
	Title            string                      `json:"title" gorm:"not null"`
	Price            Money                       `json:"price" gorm:"type:numeric(12,2);not null"`
	CompareAtPrice   *Money                      `json:"compare_at_price,omitempty" gorm:"type:numeric(12,2)"`
	Stock            int                         `json:"stock" gorm:"not null;default:0"`
	Position         int                         `json:"position" gorm:"not null;default:1"`
	IsPublished      bool                        `json:"is_published" gorm:"not null;default:true;index"`
	WeightGrams      *int                        `json:"weight_grams,omitempty"`
	LengthCm         *float64                    `json:"length_cm,omitempty"`
	WidthCm          *float64                    `json:"width_cm,omitempty"`
	HeightCm         *float64                    `json:"height_cm,omitempty"`
	OptionValueLinks []ProductVariantOptionValue `json:"option_value_links,omitempty"`
}

type ProductVariantOptionValue struct {
	BaseModel
	ProductVariantID     uint `json:"product_variant_id" gorm:"not null;index"`
	ProductOptionValueID uint `json:"product_option_value_id" gorm:"not null;index"`
}

type ProductAttribute struct {
	BaseModel
	Key        string      `json:"key" gorm:"uniqueIndex;not null"`
	Slug       string      `json:"slug" gorm:"uniqueIndex;not null"`
	Type       string      `json:"type" gorm:"not null"`
	Filterable bool        `json:"filterable" gorm:"not null;default:false;index"`
	Sortable   bool        `json:"sortable" gorm:"not null;default:false;index"`
	EnumValues StringArray `json:"enum_values,omitempty" gorm:"type:text[]"`
}

type ProductAttributeValue struct {
	BaseModel
	ProductID          uint              `json:"product_id" gorm:"not null;index"`
	ProductAttributeID uint              `json:"product_attribute_id" gorm:"not null;index"`
	TextValue          *string           `json:"text_value,omitempty"`
	NumberValue        *float64          `json:"number_value,omitempty"`
	BooleanValue       *bool             `json:"boolean_value,omitempty"`
	EnumValue          *string           `json:"enum_value,omitempty"`
	Position           int               `json:"position" gorm:"not null;default:1"`
	ProductAttribute   *ProductAttribute `json:"product_attribute,omitempty"`
}

type SEOMetadata struct {
	BaseModel
	EntityType          string  `json:"entity_type" gorm:"not null;index:idx_seo_entity,unique"`
	EntityID            uint    `json:"entity_id" gorm:"not null;index:idx_seo_entity,unique"`
	Title               *string `json:"title,omitempty"`
	Description         *string `json:"description,omitempty"`
	CanonicalPath       *string `json:"canonical_path,omitempty" gorm:"uniqueIndex"`
	OgImageMediaID      *string `json:"og_image_media_id,omitempty"`
	NoIndex             bool    `json:"noindex" gorm:"not null;default:false"`
	Robots              string  `json:"robots" gorm:"size:32;not null;default:index_follow"`
	OGTitle             *string `json:"og_title,omitempty"`
	OGDescription       *string `json:"og_description,omitempty"`
	TwitterCard         string  `json:"twitter_card" gorm:"size:32;not null;default:summary_large_image"`
	TwitterTitle        *string `json:"twitter_title,omitempty"`
	TwitterDescription  *string `json:"twitter_description,omitempty"`
	TwitterImageMediaID *string `json:"twitter_image_media_id,omitempty"`
	JSONLD              string  `json:"json_ld" gorm:"type:jsonb;not null;default:'[]'"`
}

type ProductDraft struct {
	BaseModel
	ProductID         uint                         `json:"product_id" gorm:"not null;uniqueIndex"`
	Version           int                          `json:"version" gorm:"not null;default:1"`
	SKU               string                       `json:"sku" gorm:"not null"`
	DefaultVariantSKU string                       `json:"default_variant_sku" gorm:"not null;default:''"`
	Name              string                       `json:"name" gorm:"not null"`
	Subtitle          *string                      `json:"subtitle,omitempty"`
	Description       string                       `json:"description"`
	Price             Money                        `json:"price" gorm:"type:numeric(12,2);not null"`
	Stock             int                          `json:"stock" gorm:"not null;default:0"`
	ImagesJSON        string                       `json:"-" gorm:"type:text;not null;default:'[]'"`
	BrandID           *uint                        `json:"brand_id,omitempty" gorm:"index"`
	SeoTitle          *string                      `json:"seo_title,omitempty"`
	SeoDescription    *string                      `json:"seo_description,omitempty"`
	SeoCanonicalPath  *string                      `json:"seo_canonical_path,omitempty"`
	SeoOgImageMediaID *string                      `json:"seo_og_image_media_id,omitempty"`
	SeoNoIndex        bool                         `json:"seo_noindex" gorm:"not null;default:false"`
	OptionDrafts      []ProductOptionDraft         `json:"option_drafts,omitempty"`
	VariantDrafts     []ProductVariantDraft        `json:"variant_drafts,omitempty"`
	AttributeDrafts   []ProductAttributeValueDraft `json:"attribute_drafts,omitempty"`
	RelatedDrafts     []ProductRelatedDraft        `json:"related_drafts,omitempty"`
	CategoryDrafts    []ProductCategoryDraft       `json:"category_drafts,omitempty"`
}

type ProductOptionDraft struct {
	BaseModel
	ProductDraftID        uint                      `json:"product_draft_id" gorm:"not null;index"`
	SourceProductOptionID *uint                     `json:"source_product_option_id,omitempty" gorm:"index"`
	Name                  string                    `json:"name" gorm:"not null"`
	Position              int                       `json:"position" gorm:"not null;default:1"`
	DisplayType           string                    `json:"display_type" gorm:"not null;default:select"`
	IsDeleted             bool                      `json:"is_deleted" gorm:"not null;default:false"`
	ValueDrafts           []ProductOptionValueDraft `json:"value_drafts,omitempty"`
}

type ProductOptionValueDraft struct {
	BaseModel
	ProductOptionDraftID       uint   `json:"product_option_draft_id" gorm:"not null;index"`
	SourceProductOptionValueID *uint  `json:"source_product_option_value_id,omitempty" gorm:"index"`
	Value                      string `json:"value" gorm:"not null"`
	Position                   int    `json:"position" gorm:"not null;default:1"`
	IsDeleted                  bool   `json:"is_deleted" gorm:"not null;default:false"`
}

type ProductVariantDraft struct {
	BaseModel
	ProductDraftID         uint                             `json:"product_draft_id" gorm:"not null;index"`
	SourceProductVariantID *uint                            `json:"source_product_variant_id,omitempty" gorm:"index"`
	SKU                    string                           `json:"sku" gorm:"not null"`
	Title                  string                           `json:"title" gorm:"not null"`
	Price                  Money                            `json:"price" gorm:"type:numeric(12,2);not null"`
	CompareAtPrice         *Money                           `json:"compare_at_price,omitempty" gorm:"type:numeric(12,2)"`
	Stock                  int                              `json:"stock" gorm:"not null;default:0"`
	Position               int                              `json:"position" gorm:"not null;default:1"`
	IsPublished            bool                             `json:"is_published" gorm:"not null;default:true"`
	WeightGrams            *int                             `json:"weight_grams,omitempty"`
	LengthCm               *float64                         `json:"length_cm,omitempty"`
	WidthCm                *float64                         `json:"width_cm,omitempty"`
	HeightCm               *float64                         `json:"height_cm,omitempty"`
	IsDeleted              bool                             `json:"is_deleted" gorm:"not null;default:false"`
	OptionValueDraftLinks  []ProductVariantOptionValueDraft `json:"option_value_draft_links,omitempty"`
}

type ProductVariantOptionValueDraft struct {
	BaseModel
	ProductVariantDraftID      uint   `json:"product_variant_draft_id" gorm:"not null;index"`
	ProductOptionValueDraftID  *uint  `json:"product_option_value_draft_id,omitempty" gorm:"index"`
	SourceProductOptionValueID *uint  `json:"source_product_option_value_id,omitempty" gorm:"index"`
	OptionName                 string `json:"option_name" gorm:"not null;default:''"`
	OptionValue                string `json:"option_value" gorm:"not null;default:''"`
	Position                   int    `json:"position" gorm:"not null;default:1"`
}

type ProductAttributeValueDraft struct {
	BaseModel
	ProductDraftID     uint     `json:"product_draft_id" gorm:"not null;index"`
	ProductAttributeID uint     `json:"product_attribute_id" gorm:"not null;index"`
	TextValue          *string  `json:"text_value,omitempty"`
	NumberValue        *float64 `json:"number_value,omitempty"`
	BooleanValue       *bool    `json:"boolean_value,omitempty"`
	EnumValue          *string  `json:"enum_value,omitempty"`
	Position           int      `json:"position" gorm:"not null;default:1"`
	IsDeleted          bool     `json:"is_deleted" gorm:"not null;default:false"`
}

type ProductRelatedDraft struct {
	BaseModel
	ProductDraftID   uint `json:"product_draft_id" gorm:"not null;index"`
	RelatedProductID uint `json:"related_product_id" gorm:"not null;index"`
	Position         int  `json:"position" gorm:"not null;default:1"`
}

type ProductCategoryDraft struct {
	BaseModel
	ProductDraftID uint `json:"product_draft_id" gorm:"not null;index"`
	CategoryID     uint `json:"category_id" gorm:"not null;index"`
	Position       int  `json:"position" gorm:"not null;default:1"`
}
