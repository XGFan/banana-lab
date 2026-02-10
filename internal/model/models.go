package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	PromptModeTxt2Img = "txt2img"
	PromptModeImg2Img = "img2img"

	ResultStatusActive   = "active"
	ResultStatusArchived = "archived"
	ResultStatusDeleted  = "deleted"

	AssetDomainTxt2Img = PromptModeTxt2Img
	AssetDomainImg2Img = PromptModeImg2Img

	AssetOriginGenerated = "generated"
	AssetOriginUploaded  = "uploaded"
)

type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	data, err := json.Marshal([]string(s))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (s *StringSlice) Scan(value any) error {
	if value == nil {
		*s = nil
		return nil
	}

	var bytes []byte
	switch typed := value.(type) {
	case string:
		bytes = []byte(typed)
	case []byte:
		bytes = typed
	default:
		return fmt.Errorf("unsupported StringSlice type %T", value)
	}

	if len(bytes) == 0 {
		*s = nil
		return nil
	}

	var decoded []string
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		return err
	}
	*s = decoded
	return nil
}

type PromptFragment map[string]any

type PromptFragments []PromptFragment

type PromptCollectionState map[string]any

func (f PromptFragments) Value() (driver.Value, error) {
	if f == nil {
		return nil, nil
	}
	data, err := json.Marshal([]PromptFragment(f))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (f *PromptFragments) Scan(value any) error {
	if value == nil {
		*f = nil
		return nil
	}

	var bytes []byte
	switch typed := value.(type) {
	case string:
		bytes = []byte(typed)
	case []byte:
		bytes = typed
	default:
		return fmt.Errorf("unsupported PromptFragments type %T", value)
	}

	if len(bytes) == 0 {
		*f = nil
		return nil
	}

	var decoded []map[string]any
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		return err
	}
	out := make(PromptFragments, 0, len(decoded))
	for _, item := range decoded {
		out = append(out, PromptFragment(item))
	}
	*f = out
	return nil
}

func (s PromptCollectionState) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	data, err := json.Marshal(map[string]any(s))
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (s *PromptCollectionState) Scan(value any) error {
	if value == nil {
		*s = nil
		return nil
	}

	var bytes []byte
	switch typed := value.(type) {
	case string:
		bytes = []byte(typed)
	case []byte:
		bytes = typed
	default:
		return fmt.Errorf("unsupported PromptCollectionState type %T", value)
	}

	if len(bytes) == 0 {
		*s = nil
		return nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		return err
	}
	*s = PromptCollectionState(decoded)
	return nil
}

func (s PromptCollectionState) Clone() PromptCollectionState {
	if s == nil {
		return nil
	}
	data, err := json.Marshal(map[string]any(s))
	if err != nil {
		return PromptCollectionState{}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return PromptCollectionState{}
	}
	return PromptCollectionState(out)
}

type PromptNode struct {
	ID                string                `gorm:"primaryKey" json:"id"`
	ParentID          *string               `gorm:"column:parent_id;index" json:"parent_id,omitempty"`
	Mode              string                `gorm:"column:mode;not null;index:idx_prompt_nodes_mode_pinned_last_used,priority:1" json:"mode"`
	Prompt            string                `gorm:"column:prompt;not null" json:"prompt"`
	Fragments         PromptCollectionState `gorm:"column:fragments;type:text" json:"fragments"`
	Title             string                `gorm:"column:title;not null" json:"title"`
	Pinned            bool                  `gorm:"column:pinned;not null;default:false;index:idx_prompt_nodes_mode_pinned_last_used,priority:2" json:"pinned"`
	SortOrder         *int                  `gorm:"column:sort_order;index" json:"sort_order,omitempty"`
	LastUsedAt        time.Time             `gorm:"column:last_used_at;not null;index:idx_prompt_nodes_mode_pinned_last_used,priority:3" json:"last_used_at"`
	ArchivedAt        *time.Time            `gorm:"column:archived_at;index" json:"archived_at,omitempty"`
	NegativePrompt    *string               `gorm:"column:negative_prompt" json:"negative_prompt,omitempty"`
	CfgScale          *float64              `gorm:"column:cfg_scale" json:"cfg_scale,omitempty"`
	AspectRatio       *string               `gorm:"column:aspect_ratio" json:"aspect_ratio,omitempty"`
	ImageSize         *string               `gorm:"column:image_size" json:"image_size,omitempty"`
	Seed              *int64                `gorm:"column:seed" json:"seed,omitempty"`
	Steps             *int                  `gorm:"column:steps" json:"steps,omitempty"`
	ReferenceImageIDs StringSlice           `gorm:"column:reference_image_ids;type:text" json:"reference_image_ids,omitempty"`
	CreatedAt         time.Time             `gorm:"column:created_at" json:"created_at"`
	UpdatedAt         time.Time             `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt         *time.Time            `gorm:"column:deleted_at;index" json:"deleted_at,omitempty"`
}

func (PromptNode) TableName() string {
	return "prompt_nodes"
}

// Node is kept as alias for incremental API refactor.
type Node = PromptNode

type PromptSnippet struct {
	ID               string          `gorm:"primaryKey" json:"id"`
	Name             string          `gorm:"column:name;not null;index" json:"name"`
	ContentFragments PromptFragments `gorm:"column:content_fragments;type:text" json:"content_fragments"`
	CreatedAt        time.Time       `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time       `gorm:"column:updated_at" json:"updated_at"`
}

func (PromptSnippet) TableName() string {
	return "prompt_snippets"
}

type Result struct {
	ID            string     `gorm:"primaryKey" json:"id"`
	NodeID        string     `gorm:"column:node_id;not null;index" json:"node_id"`
	AssetID       string     `gorm:"column:asset_id;not null;index" json:"asset_id"`
	Status        string     `gorm:"column:status;not null;default:active;index" json:"status"`
	SourceAssetID *string    `gorm:"column:source_asset_id;index" json:"source_asset_id,omitempty"`
	SourceDomain  *string    `gorm:"column:source_domain" json:"source_domain,omitempty"`
	ArchivedAt    *time.Time `gorm:"column:archived_at" json:"archived_at,omitempty"`
	DeletedAt     *time.Time `gorm:"column:deleted_at" json:"deleted_at,omitempty"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (r *Result) BeforeCreate(_ *gorm.DB) error {
	if r.Status == "" {
		r.Status = ResultStatusActive
	}
	return nil
}

type Asset struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Domain    string    `gorm:"column:domain;not null;index" json:"domain"`
	Origin    string    `gorm:"column:origin;not null;default:generated;index" json:"origin"`
	FilePath  string    `gorm:"column:file_path;not null" json:"file_path"`
	FileSize  *int64    `gorm:"column:file_size" json:"file_size,omitempty"`
	Width     *int      `gorm:"column:width" json:"width,omitempty"`
	Height    *int      `gorm:"column:height" json:"height,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

type Image struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	NodeID    *string   `gorm:"column:node_id;index" json:"node_id,omitempty"`
	Origin    string    `gorm:"column:origin;not null;default:generated;index" json:"origin"`
	FilePath  string    `gorm:"column:file_path" json:"file_path"`
	FileSize  *int64    `gorm:"column:file_size" json:"file_size,omitempty"`
	Width     *int      `gorm:"column:width" json:"width,omitempty"`
	Height    *int      `gorm:"column:height" json:"height,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}
