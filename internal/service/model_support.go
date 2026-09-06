package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cpa-usage/internal/cpa/dto/authfiles"
	cpamodels "cpa-usage/internal/cpa/dto/models"
	"cpa-usage/internal/cpa/dto/response"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"

	"gorm.io/gorm"
)

const (
	ModelSupportMaxAccounts        = 12
	ModelSupportMaxConcurrency     = 4
	ModelSupportTimeout            = 15 * time.Second
	ModelSupportMaxRequestsPerItem = 3
)

var (
	ErrModelSupportScopeRequired = errors.New("model support account scope is required")
	ErrModelSupportScopeTooLarge = errors.New("model support account scope is too large")
	ErrModelSupportScopeInvalid  = errors.New("model support account scope is invalid")
)

type ModelSupportClient interface {
	FetchAuthFileByAuthIndex(context.Context, string) (authfiles.AuthFile, bool, error)
	FetchAuthFileModels(context.Context, string) (*response.AuthFileModelsResult, error)
	FetchStaticModelDefinitions(context.Context, string) (*response.StaticModelDefinitionsResult, error)
}

type ModelSupportService struct {
	db          *gorm.DB
	client      ModelSupportClient
	timeout     time.Duration
	concurrency int
}

type ModelSupportRequest struct {
	IdentityIDs []uint
}

type ModelSupportLimits struct {
	MaxAccounts         int
	MaxConcurrency      int
	TimeoutSeconds      int
	MaxUpstreamRequests int
}

type ModelSupportResult struct {
	Complete      bool
	SelectedCount int
	LoadedCount   int
	Accounts      []AccountModelSupport
	Models        []ModelSupportCoverage
	Limits        ModelSupportLimits
}

type AccountModelSupport struct {
	IdentityID       uint
	AuthIndex        string
	DisplayName      string
	Provider         string
	Channel          string
	Disabled         bool
	Unavailable      *bool
	Status           string
	ErrorCode        string
	CatalogStatus    string
	RegisteredModels []RegisteredModelSupport
}

type RegisteredModelSupport struct {
	ID               string
	DisplayName      string
	Type             string
	OwnedBy          string
	DefinitionStatus string
	Capability       *ModelCapability
}

type ModelCapability struct {
	ContextLength             *int
	InputTokenLimit           *int
	MaxCompletionTokens       *int
	OutputTokenLimit          *int
	SupportedInputModalities  []string
	SupportedOutputModalities []string
	Thinking                  *ModelThinkingSupport
}

type ModelThinkingSupport struct {
	Min            *int
	Max            *int
	ZeroAllowed    *bool
	DynamicAllowed *bool
	Levels         []string
}

type ModelSupportCoverage struct {
	ModelID                        string
	DisplayName                    string
	ObservedSupportingAccounts     int
	SelectedAccounts               int
	SingleRegisteredAccountInScope *bool
}

type modelDefinitionCatalog struct {
	status      string
	definitions map[string]cpamodels.StaticModelDefinition
}

func NewModelSupportService(db *gorm.DB, client ModelSupportClient) *ModelSupportService {
	return &ModelSupportService{db: db, client: client, timeout: ModelSupportTimeout, concurrency: ModelSupportMaxConcurrency}
}

// Load performs one bounded, request-scoped read. It does not persist results,
// retry failed calls, or create background work.
func (s *ModelSupportService) Load(ctx context.Context, request ModelSupportRequest) (ModelSupportResult, error) {
	result := ModelSupportResult{Limits: defaultModelSupportLimits()}
	if s == nil || s.db == nil || s.client == nil {
		return result, fmt.Errorf("model support service is not configured")
	}
	ids, err := validateModelSupportScope(request.IdentityIDs)
	if err != nil {
		return result, err
	}
	identities, err := loadModelSupportIdentities(ctx, s.db, ids)
	if err != nil {
		return result, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	channels := uniqueModelSupportChannels(identities)
	catalogs := s.loadDefinitionCatalogs(requestCtx, channels)
	accounts := s.loadAccounts(requestCtx, identities, catalogs)

	loaded := 0
	for _, account := range accounts {
		if account.Status == "loaded" {
			loaded++
		}
	}
	result.Complete = loaded == len(accounts)
	result.SelectedCount = len(accounts)
	result.LoadedCount = loaded
	result.Accounts = accounts
	result.Models = buildModelSupportCoverage(accounts, result.Complete)
	return result, nil
}

func defaultModelSupportLimits() ModelSupportLimits {
	return ModelSupportLimits{
		MaxAccounts:         ModelSupportMaxAccounts,
		MaxConcurrency:      ModelSupportMaxConcurrency,
		TimeoutSeconds:      int(ModelSupportTimeout / time.Second),
		MaxUpstreamRequests: ModelSupportMaxAccounts * ModelSupportMaxRequestsPerItem,
	}
}

func validateModelSupportScope(raw []uint) ([]uint, error) {
	if len(raw) == 0 {
		return nil, ErrModelSupportScopeRequired
	}
	if len(raw) > ModelSupportMaxAccounts {
		return nil, fmt.Errorf("%w: select at most %d accounts", ErrModelSupportScopeTooLarge, ModelSupportMaxAccounts)
	}
	ids := make([]uint, 0, len(raw))
	seen := make(map[uint]struct{}, len(raw))
	for _, id := range raw {
		if id == 0 {
			return nil, fmt.Errorf("%w: account id must be positive", ErrModelSupportScopeInvalid)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("%w: duplicate account id %d", ErrModelSupportScopeInvalid, id)
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func loadModelSupportIdentities(ctx context.Context, db *gorm.DB, ids []uint) ([]entities.UsageIdentity, error) {
	identities := make([]entities.UsageIdentity, 0, len(ids))
	for _, id := range ids {
		identity, err := repository.GetUsageIdentityByID(ctx, db, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("%w: account %d was not found", ErrModelSupportScopeInvalid, id)
			}
			return nil, fmt.Errorf("load model support account %d: %w", id, err)
		}
		if identity.IsDeleted || identity.AuthType != entities.UsageIdentityAuthTypeAuthFile || strings.TrimSpace(identity.Identity) == "" {
			return nil, fmt.Errorf("%w: account %d is not an active auth-file identity", ErrModelSupportScopeInvalid, id)
		}
		identities = append(identities, identity)
	}
	return identities, nil
}

func uniqueModelSupportChannels(identities []entities.UsageIdentity) []string {
	seen := make(map[string]struct{}, len(identities))
	channels := make([]string, 0, len(identities))
	for _, identity := range identities {
		channel, ok := modelDefinitionChannel(identity.Type)
		if !ok {
			continue
		}
		if _, exists := seen[channel]; exists {
			continue
		}
		seen[channel] = struct{}{}
		channels = append(channels, channel)
	}
	sort.Strings(channels)
	return channels
}

func modelDefinitionChannel(providerType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "claude":
		return "claude", true
	case "gemini", "gemini-cli":
		return "gemini", true
	case "gemini-interactions":
		return "gemini-interactions", true
	case "vertex":
		return "vertex", true
	case "aistudio":
		return "aistudio", true
	case "codex":
		return "codex", true
	case "kimi":
		return "kimi", true
	case "antigravity":
		return "antigravity", true
	case "xai", "x-ai", "grok":
		return "xai", true
	default:
		return "", false
	}
}

func (s *ModelSupportService) loadDefinitionCatalogs(ctx context.Context, channels []string) map[string]modelDefinitionCatalog {
	catalogs := make(map[string]modelDefinitionCatalog, len(channels))
	var mu sync.Mutex
	s.runBounded(len(channels), func(index int) {
		channel := channels[index]
		catalog := modelDefinitionCatalog{status: "error"}
		response, err := s.client.FetchStaticModelDefinitions(ctx, channel)
		if err == nil && response != nil && strings.EqualFold(strings.TrimSpace(response.Payload.Channel), channel) {
			definitions, valid := indexStaticModelDefinitions(response.Payload.Models)
			if valid {
				catalog.status = "loaded"
				catalog.definitions = definitions
			}
		}
		mu.Lock()
		catalogs[channel] = catalog
		mu.Unlock()
	})
	return catalogs
}

func indexStaticModelDefinitions(items []cpamodels.StaticModelDefinition) (map[string]cpamodels.StaticModelDefinition, bool) {
	definitions := make(map[string]cpamodels.StaticModelDefinition, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			return nil, false
		}
		if _, duplicate := definitions[id]; duplicate {
			return nil, false
		}
		item.ID = id
		definitions[id] = item
	}
	return definitions, true
}

func (s *ModelSupportService) loadAccounts(ctx context.Context, identities []entities.UsageIdentity, catalogs map[string]modelDefinitionCatalog) []AccountModelSupport {
	accounts := make([]AccountModelSupport, len(identities))
	s.runBounded(len(identities), func(index int) {
		accounts[index] = s.loadAccount(ctx, identities[index], catalogs)
	})
	return accounts
}

func (s *ModelSupportService) loadAccount(ctx context.Context, identity entities.UsageIdentity, catalogs map[string]modelDefinitionCatalog) AccountModelSupport {
	channel, channelKnown := modelDefinitionChannel(identity.Type)
	account := AccountModelSupport{
		IdentityID:  identity.ID,
		AuthIndex:   identity.Identity,
		DisplayName: identity.DisplayName(),
		Provider:    strings.TrimSpace(identity.Provider),
		Channel:     channel,
		Disabled:    identity.Disabled,
		Unavailable: copyOptionalBool(identity.Unavailable),
		Status:      "failed",
	}
	if !channelKnown {
		account.CatalogStatus = "unknown_channel"
	} else {
		account.CatalogStatus = catalogs[channel].status
	}

	file, found, err := s.client.FetchAuthFileByAuthIndex(ctx, identity.Identity)
	if err != nil {
		account.ErrorCode = "upstream_error"
		return account
	}
	lookupName := strings.TrimSpace(file.ID)
	if lookupName == "" {
		lookupName = strings.TrimSpace(file.Name)
	}
	if !found || lookupName == "" {
		account.ErrorCode = "auth_file_missing"
		return account
	}
	if strings.TrimSpace(file.AuthIndex) != strings.TrimSpace(identity.Identity) {
		account.ErrorCode = "invalid_upstream_response"
		return account
	}
	registered, err := s.client.FetchAuthFileModels(ctx, lookupName)
	if err != nil || registered == nil {
		account.ErrorCode = "upstream_error"
		return account
	}
	models, valid := buildRegisteredModelSupport(registered.Payload.Models, catalogs[channel], channelKnown)
	if !valid {
		account.ErrorCode = "invalid_upstream_response"
		return account
	}
	account.Status = "loaded"
	account.RegisteredModels = models
	return account
}

func buildRegisteredModelSupport(items []cpamodels.RegisteredModel, catalog modelDefinitionCatalog, channelKnown bool) ([]RegisteredModelSupport, bool) {
	models := make([]RegisteredModelSupport, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			return nil, false
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		model := RegisteredModelSupport{
			ID:          id,
			DisplayName: strings.TrimSpace(item.DisplayName),
			Type:        strings.TrimSpace(item.Type),
			OwnedBy:     strings.TrimSpace(item.OwnedBy),
		}
		switch {
		case !channelKnown:
			model.DefinitionStatus = "unknown_channel"
		case catalog.status != "loaded":
			model.DefinitionStatus = "error"
		case catalog.definitions[id].ID == "":
			model.DefinitionStatus = "absent"
		default:
			model.DefinitionStatus = "available"
			model.Capability = modelCapability(catalog.definitions[id])
		}
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, true
}

func modelCapability(item cpamodels.StaticModelDefinition) *ModelCapability {
	capability := &ModelCapability{
		ContextLength:             copyOptionalInt(item.ContextLength),
		InputTokenLimit:           copyOptionalInt(item.InputTokenLimit),
		MaxCompletionTokens:       copyOptionalInt(item.MaxCompletionTokens),
		OutputTokenLimit:          copyOptionalInt(item.OutputTokenLimit),
		SupportedInputModalities:  append([]string(nil), item.SupportedInputModalities...),
		SupportedOutputModalities: append([]string(nil), item.SupportedOutputModalities...),
	}
	if item.Thinking != nil {
		capability.Thinking = &ModelThinkingSupport{
			Min:            copyOptionalInt(item.Thinking.Min),
			Max:            copyOptionalInt(item.Thinking.Max),
			ZeroAllowed:    copyOptionalBool(item.Thinking.ZeroAllowed),
			DynamicAllowed: copyOptionalBool(item.Thinking.DynamicAllowed),
			Levels:         append([]string(nil), item.Thinking.Levels...),
		}
	}
	if capability.ContextLength == nil && capability.InputTokenLimit == nil && capability.MaxCompletionTokens == nil && capability.OutputTokenLimit == nil && len(capability.SupportedInputModalities) == 0 && len(capability.SupportedOutputModalities) == 0 && capability.Thinking == nil {
		return nil
	}
	return capability
}

func buildModelSupportCoverage(accounts []AccountModelSupport, complete bool) []ModelSupportCoverage {
	type aggregate struct {
		displayName string
		count       int
	}
	byModel := make(map[string]aggregate)
	for _, account := range accounts {
		if account.Status != "loaded" {
			continue
		}
		for _, model := range account.RegisteredModels {
			item := byModel[model.ID]
			if item.displayName == "" {
				item.displayName = model.DisplayName
			}
			item.count++
			byModel[model.ID] = item
		}
	}
	models := make([]ModelSupportCoverage, 0, len(byModel))
	for id, item := range byModel {
		coverage := ModelSupportCoverage{
			ModelID:                    id,
			DisplayName:                item.displayName,
			ObservedSupportingAccounts: item.count,
			SelectedAccounts:           len(accounts),
		}
		if complete {
			single := item.count == 1
			coverage.SingleRegisteredAccountInScope = &single
		}
		models = append(models, coverage)
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].ObservedSupportingAccounts != models[j].ObservedSupportingAccounts {
			return models[i].ObservedSupportingAccounts > models[j].ObservedSupportingAccounts
		}
		return models[i].ModelID < models[j].ModelID
	})
	return models
}

func (s *ModelSupportService) runBounded(count int, run func(int)) {
	if count == 0 {
		return
	}
	limit := s.concurrency
	if limit <= 0 || limit > ModelSupportMaxConcurrency {
		limit = ModelSupportMaxConcurrency
	}
	sem := make(chan struct{}, limit)
	var wait sync.WaitGroup
	wait.Add(count)
	for index := 0; index < count; index++ {
		go func(index int) {
			defer wait.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			run(index)
		}(index)
	}
	wait.Wait()
}

func copyOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
