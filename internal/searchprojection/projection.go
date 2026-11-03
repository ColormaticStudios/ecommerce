// Package searchprojection defines the frozen, rebuildable SQL retrieval projection.
package searchprojection

import (
	"encoding/json"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Rank struct {
	ProductID   uint    `gorm:"primaryKey;autoIncrement:false"`
	Searchable  string  `gorm:"type:text;not null"`
	Name        string  `gorm:"type:text;not null"`
	Description string  `gorm:"type:text;not null"`
	Brand       string  `gorm:"type:text;not null"`
	Attributes  string  `gorm:"type:text;not null"`
	BrandLabel  string  `gorm:"type:text;not null"`
	NameLabel   string  `gorm:"type:text;not null"`
	RecencyBase float64 `gorm:"not null"`
}

func (Rank) TableName() string { return "search_product_ranks" }

type Category struct {
	ProductID uint   `gorm:"primaryKey;autoIncrement:false"`
	Slug      string `gorm:"primaryKey;type:text"`
	Label     string `gorm:"type:text;not null"`
}

func (Category) TableName() string { return "search_product_categories" }

type Attribute struct {
	ProductID uint   `gorm:"primaryKey;autoIncrement:false"`
	Slug      string `gorm:"primaryKey;type:text"`
	Value     string `gorm:"primaryKey;type:text"`
	Label     string `gorm:"type:text;not null"`
	Name      string `gorm:"type:text;not null"`
}

func (Attribute) TableName() string { return "search_product_attributes" }

type Term struct {
	ProductID uint   `gorm:"primaryKey;autoIncrement:false;index:idx_search_product_terms_vocabulary,priority:3;index:idx_search_product_terms_lookup,priority:2"`
	Term      string `gorm:"primaryKey;type:text;index;index:idx_search_product_terms_vocabulary,priority:2;index:idx_search_product_terms_lookup,priority:1"`
	Length    int    `gorm:"not null;index;index:idx_search_product_terms_vocabulary,priority:1"`
}

func (Term) TableName() string { return "search_product_terms" }

// Input deliberately does not depend on evolving catalog or search models. It
// can replay historical JSON without selecting columns introduced later.
type Input struct {
	ProductID                       uint
	Payload, Searchable, Categories string
	Created                         time.Time
	Active                          bool
}
type payload struct {
	Name, Description string
	Brand             *struct{ Name string }
	Categories        []struct{ Slug, Name string }
	AttributeValues   []struct {
		ProductAttribute *struct {
			Slug, Key  string
			Filterable bool
		} `json:"product_attribute"`
		TextValue    *string  `json:"text_value"`
		EnumValue    *string  `json:"enum_value"`
		NumberValue  *float64 `json:"number_value"`
		BooleanValue *bool    `json:"boolean_value"`
	} `json:"attribute_values"`
}

func normalize(s string) string {
	var out strings.Builder
	space := true
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			out.WriteRune(r)
			space = false
		} else if !space {
			out.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(out.String())
}
func Replace(tx *gorm.DB, in Input) error {
	for _, table := range []string{"search_product_ranks", "search_product_categories", "search_product_attributes", "search_product_terms"} {
		if err := tx.Table(table).Where("product_id = ?", in.ProductID).Delete(map[string]any{}).Error; err != nil {
			return err
		}
	}
	if !in.Active {
		return nil
	}
	var p payload
	if err := json.Unmarshal([]byte(in.Payload), &p); err != nil {
		return err
	}
	pad := func(s string) string { return " " + normalize(s) + " " }
	rank := Rank{ProductID: in.ProductID, Searchable: pad(in.Searchable), Name: pad(p.Name), Description: pad(p.Description), NameLabel: p.Name, RecencyBase: math.Exp2((float64(in.Created.Unix()) + float64(in.Created.Nanosecond())/1e9) / (30 * 86400))}
	if p.Brand != nil {
		rank.Brand = pad(p.Brand.Name)
		rank.BrandLabel = p.Brand.Name
	} else {
		rank.Brand = "  "
	}
	cats := map[string]string{}
	for _, slug := range strings.Split(in.Categories, "|") {
		if slug != "" {
			cats[slug] = slug
		}
	}
	for _, c := range p.Categories {
		slug := normalize(c.Slug)
		if _, ok := cats[slug]; ok {
			cats[slug] = c.Name
		}
	}
	for slug, label := range cats {
		if err := tx.Create(&Category{in.ProductID, slug, label}).Error; err != nil {
			return err
		}
	}
	attrs := map[string]Attribute{}
	text := []string{}
	for _, v := range p.AttributeValues {
		raw := ""
		switch {
		case v.TextValue != nil:
			raw = *v.TextValue
		case v.NumberValue != nil:
			raw = strconv.FormatFloat(*v.NumberValue, 'f', -1, 64)
		case v.BooleanValue != nil:
			raw = strconv.FormatBool(*v.BooleanValue)
		case v.EnumValue != nil:
			raw = *v.EnumValue
		}
		text = append(text, raw)
		if v.ProductAttribute == nil || !v.ProductAttribute.Filterable {
			continue
		}
		slug, value := normalize(v.ProductAttribute.Slug), normalize(raw)
		if slug != "" && value != "" {
			attrs[slug+"\x00"+value] = Attribute{in.ProductID, slug, value, raw, v.ProductAttribute.Key}
		}
	}
	rank.Attributes = pad(strings.Join(text, " "))
	if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&rank).Error; err != nil {
		return err
	}
	for _, a := range attrs {
		if err := tx.Create(&a).Error; err != nil {
			return err
		}
	}
	terms := map[string]bool{}
	rows := []Term{}
	for _, term := range strings.Fields(normalize(in.Searchable)) {
		if !terms[term] {
			terms[term] = true
			rows = append(rows, Term{in.ProductID, term, utf8.RuneCountInString(term)})
		}
	}
	if len(rows) > 0 {
		return tx.CreateInBatches(rows, 250).Error
	}
	return nil
}
