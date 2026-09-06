package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"cpa-usage/internal/cpa/dto/authfiles"
	cpamodels "cpa-usage/internal/cpa/dto/models"
	"cpa-usage/internal/cpa/dto/response"
	"cpa-usage/internal/entities"
)

type modelSupportClientStub struct {
	mu              sync.Mutex
	files           map[string]authfiles.AuthFile
	models          map[string][]cpamodels.RegisteredModel
	modelErrors     map[string]error
	definitions     map[string][]cpamodels.StaticModelDefinition
	definitionError map[string]error
	requestCount    int
	activeCalls     int
	maxActiveCalls  int
	block           <-chan struct{}
	definitionBlock <-chan struct{}
	delay           time.Duration
}

func (s *modelSupportClientStub) begin(ctx context.Context) error {
	return s.beginWithBlock(ctx, s.block)
}

func (s *modelSupportClientStub) beginWithBlock(ctx context.Context, block <-chan struct{}) error {
	s.mu.Lock()
	s.requestCount++
	s.activeCalls++
	if s.activeCalls > s.maxActiveCalls {
		s.maxActiveCalls = s.activeCalls
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.activeCalls--
		s.mu.Unlock()
	}()
	if block == nil {
		if s.delay == 0 {
			return nil
		}
		select {
		case <-time.After(s.delay):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	select {
	case <-block:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *modelSupportClientStub) FetchAuthFileByAuthIndex(ctx context.Context, authIndex string) (authfiles.AuthFile, bool, error) {
	if err := s.begin(ctx); err != nil {
		return authfiles.AuthFile{}, false, err
	}
	file, ok := s.files[authIndex]
	return file, ok, nil
}

func (s *modelSupportClientStub) FetchAuthFileModels(ctx context.Context, name string) (*response.AuthFileModelsResult, error) {
	if err := s.begin(ctx); err != nil {
		return nil, err
	}
	if err := s.modelErrors[name]; err != nil {
		return nil, err
	}
	return &response.AuthFileModelsResult{Payload: cpamodels.AuthFileModelsResponse{Models: s.models[name]}}, nil
}

func (s *modelSupportClientStub) FetchStaticModelDefinitions(ctx context.Context, channel string) (*response.StaticModelDefinitionsResult, error) {
	block := s.block
	if s.definitionBlock != nil {
		block = s.definitionBlock
	}
	if err := s.beginWithBlock(ctx, block); err != nil {
		return nil, err
	}
	if err := s.definitionError[channel]; err != nil {
		return nil, err
	}
	return &response.StaticModelDefinitionsResult{Payload: cpamodels.StaticModelDefinitionsResponse{Channel: channel, Models: s.definitions[channel]}}, nil
}

func TestLoadModelSupportKeepsRegisteredSupportWhenCatalogTimesOut(t *testing.T) {
	db := openSyncTestDatabase(t)
	identity := entities.UsageIdentity{Name: "Codex", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth-a", Type: "codex", Provider: "Codex"}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("seed identity: %v", err)
	}
	blocked := make(chan struct{})
	client := &modelSupportClientStub{
		files:       map[string]authfiles.AuthFile{"auth-a": {AuthIndex: "auth-a", Name: "a.json"}},
		models:      map[string][]cpamodels.RegisteredModel{"a.json": {{ID: "gpt-exact"}}},
		modelErrors: map[string]error{}, definitions: map[string][]cpamodels.StaticModelDefinition{},
		definitionError: map[string]error{}, definitionBlock: blocked,
	}
	service := NewModelSupportService(db, client)
	service.timeout = 10 * time.Millisecond
	result, err := service.Load(context.Background(), ModelSupportRequest{IdentityIDs: []uint{identity.ID}})
	if err != nil {
		t.Fatalf("catalog timeout should be represented as optional enrichment failure: %v", err)
	}
	if !result.Complete || result.LoadedCount != 1 || result.Accounts[0].Status != "loaded" {
		t.Fatalf("catalog timeout must not erase registered support: %+v", result)
	}
	if result.Accounts[0].CatalogStatus != "error" || len(result.Accounts[0].RegisteredModels) != 1 || result.Accounts[0].RegisteredModels[0].DefinitionStatus != "error" {
		t.Fatalf("expected explicit catalog-only failure: %+v", result.Accounts[0])
	}
}

func TestLoadModelSupportRejectsMissingModelCollectionsButAcceptsEmptyArrays(t *testing.T) {
	db := openSyncTestDatabase(t)
	identities := []entities.UsageIdentity{
		{Name: "Missing", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth-missing", Type: "codex", Provider: "Codex"},
		{Name: "Empty", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth-empty", Type: "claude", Provider: "Claude"},
	}
	if err := db.Create(&identities).Error; err != nil {
		t.Fatalf("seed identities: %v", err)
	}
	client := &modelSupportClientStub{
		files: map[string]authfiles.AuthFile{
			"auth-missing": {AuthIndex: "auth-missing", Name: "missing.json"},
			"auth-empty":   {AuthIndex: "auth-empty", Name: "empty.json"},
		},
		models: map[string][]cpamodels.RegisteredModel{
			"missing.json": nil,
			"empty.json":   {},
		},
		modelErrors: map[string]error{},
		definitions: map[string][]cpamodels.StaticModelDefinition{
			"claude": {},
		},
		definitionError: map[string]error{},
	}
	service := NewModelSupportService(db, client)
	result, err := service.Load(context.Background(), ModelSupportRequest{IdentityIDs: []uint{identities[0].ID, identities[1].ID}})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if result.Complete || result.Accounts[0].ErrorCode != "invalid_upstream_response" {
		t.Fatalf("missing/null registered models must be invalid: %+v", result.Accounts[0])
	}
	if result.Accounts[1].Status != "loaded" || result.Accounts[1].CatalogStatus != "loaded" || result.Accounts[1].RegisteredModels == nil {
		t.Fatalf("explicit empty arrays must remain valid: %+v", result.Accounts[1])
	}

	client.models["missing.json"] = []cpamodels.RegisteredModel{{ID: "gpt-exact"}}
	client.definitions["codex"] = nil
	result, err = service.Load(context.Background(), ModelSupportRequest{IdentityIDs: []uint{identities[0].ID}})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !result.Complete || result.Accounts[0].CatalogStatus != "error" || result.Accounts[0].RegisteredModels[0].DefinitionStatus != "error" {
		t.Fatalf("missing/null static models must be explicit enrichment failure: %+v", result.Accounts[0])
	}
}

func TestLoadModelSupportReturnsCompleteExactCoverageAndCapabilities(t *testing.T) {
	db := openSyncTestDatabase(t)
	unavailable := true
	identities := []entities.UsageIdentity{{
		Name: "Codex A", AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "auth-a", Type: "codex", Provider: "Codex",
	}, {
		Name: "Codex B", AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "auth-b", Type: "codex", Provider: "Codex", Disabled: true, Unavailable: &unavailable,
	}}
	if err := db.Create(&identities).Error; err != nil {
		t.Fatalf("seed identities: %v", err)
	}
	contextLength := 200000
	zeroAllowed := false
	client := &modelSupportClientStub{
		files: map[string]authfiles.AuthFile{
			"auth-a": {ID: "auth-id-a", AuthIndex: "auth-a", Name: "shared.json"},
			"auth-b": {AuthIndex: "auth-b", Name: "b.json"},
		},
		models: map[string][]cpamodels.RegisteredModel{
			"auth-id-a": {{ID: "shared", DisplayName: "Shared"}, {ID: "solo", DisplayName: "Solo"}, {ID: "missing-static"}},
			"b.json":    {{ID: "shared", DisplayName: "Shared"}},
		},
		definitions: map[string][]cpamodels.StaticModelDefinition{
			"codex": {{ID: "shared", ContextLength: &contextLength, SupportedInputModalities: []string{"TEXT", "IMAGE"}, Thinking: &cpamodels.ThinkingSupport{ZeroAllowed: &zeroAllowed}}, {ID: "solo"}},
		},
		modelErrors: map[string]error{}, definitionError: map[string]error{},
	}
	service := NewModelSupportService(db, client)
	result, err := service.Load(context.Background(), ModelSupportRequest{IdentityIDs: []uint{identities[0].ID, identities[1].ID}})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !result.Complete || result.SelectedCount != 2 || result.LoadedCount != 2 || len(result.Accounts) != 2 {
		t.Fatalf("unexpected scope result: %+v", result)
	}
	if !result.Accounts[1].Disabled || result.Accounts[1].Unavailable == nil || !*result.Accounts[1].Unavailable {
		t.Fatalf("expected disabled/unavailable observations to stay separate: %+v", result.Accounts[1])
	}
	registered := make(map[string]RegisteredModelSupport, len(result.Accounts[0].RegisteredModels))
	for _, item := range result.Accounts[0].RegisteredModels {
		registered[item.ID] = item
	}
	if registered["shared"].Capability == nil || registered["shared"].Capability.ContextLength == nil {
		t.Fatalf("expected exact static capability match: %+v", result.Accounts[0])
	}
	if registered["missing-static"].DefinitionStatus != "absent" {
		t.Fatalf("expected unmatched exact ID to stay absent, got %+v", registered["missing-static"])
	}
	coverage := make(map[string]ModelSupportCoverage, len(result.Models))
	for _, item := range result.Models {
		coverage[item.ModelID] = item
	}
	if coverage["shared"].ObservedSupportingAccounts != 2 || coverage["shared"].SingleRegisteredAccountInScope == nil || *coverage["shared"].SingleRegisteredAccountInScope {
		t.Fatalf("unexpected shared coverage: %+v", coverage["shared"])
	}
	if coverage["solo"].SingleRegisteredAccountInScope == nil || !*coverage["solo"].SingleRegisteredAccountInScope {
		t.Fatalf("expected complete-scope single-account conclusion: %+v", coverage["solo"])
	}
	if client.requestCount != 5 {
		t.Fatalf("expected one catalog plus two bounded account lookups, got %d requests", client.requestCount)
	}
}

func TestLoadModelSupportMarksAccountFailurePartialWithoutSingleAccountConclusion(t *testing.T) {
	db := openSyncTestDatabase(t)
	identities := []entities.UsageIdentity{{
		Name: "Codex A", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth-a", Type: "codex", Provider: "Codex",
	}, {
		Name: "Codex B", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth-b", Type: "codex", Provider: "Codex",
	}}
	if err := db.Create(&identities).Error; err != nil {
		t.Fatalf("seed identities: %v", err)
	}
	client := &modelSupportClientStub{
		files:           map[string]authfiles.AuthFile{"auth-a": {AuthIndex: "auth-a", Name: "a.json"}, "auth-b": {AuthIndex: "auth-b", Name: "b.json"}},
		models:          map[string][]cpamodels.RegisteredModel{"a.json": {{ID: "solo"}}},
		modelErrors:     map[string]error{"b.json": errors.New("provider unavailable")},
		definitions:     map[string][]cpamodels.StaticModelDefinition{"codex": {{ID: "solo"}}},
		definitionError: map[string]error{},
	}
	result, err := NewModelSupportService(db, client).Load(context.Background(), ModelSupportRequest{IdentityIDs: []uint{identities[0].ID, identities[1].ID}})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if result.Complete || result.LoadedCount != 1 || result.Accounts[1].ErrorCode != "upstream_error" {
		t.Fatalf("expected visible partial result, got %+v", result)
	}
	if len(result.Models) != 1 || result.Models[0].SingleRegisteredAccountInScope != nil {
		t.Fatalf("partial scope must not claim a single supporting account: %+v", result.Models)
	}
}

func TestLoadModelSupportKeepsUnknownChannelSeparateFromRegisteredSupport(t *testing.T) {
	db := openSyncTestDatabase(t)
	identity := entities.UsageIdentity{Name: "Plugin", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth-plugin", Type: "plugin-provider", Provider: "Plugin"}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("seed identity: %v", err)
	}
	client := &modelSupportClientStub{
		files:       map[string]authfiles.AuthFile{"auth-plugin": {AuthIndex: "auth-plugin", Name: "plugin.json"}},
		models:      map[string][]cpamodels.RegisteredModel{"plugin.json": {{ID: "plugin-model"}}},
		modelErrors: map[string]error{}, definitions: map[string][]cpamodels.StaticModelDefinition{}, definitionError: map[string]error{},
	}
	result, err := NewModelSupportService(db, client).Load(context.Background(), ModelSupportRequest{IdentityIDs: []uint{identity.ID}})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !result.Complete || result.Accounts[0].CatalogStatus != "unknown_channel" || result.Accounts[0].RegisteredModels[0].DefinitionStatus != "unknown_channel" {
		t.Fatalf("unexpected unknown-channel result: %+v", result)
	}
	if client.requestCount != 2 {
		t.Fatalf("unknown channel should not call static definitions, got %d requests", client.requestCount)
	}
}

func TestLoadModelSupportKeepsCatalogFailureSeparateFromCompleteRegisteredSupport(t *testing.T) {
	db := openSyncTestDatabase(t)
	identity := entities.UsageIdentity{Name: "Codex", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth-codex", Type: "codex", Provider: "Codex"}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("seed identity: %v", err)
	}
	client := &modelSupportClientStub{
		files:  map[string]authfiles.AuthFile{"auth-codex": {AuthIndex: "auth-codex", Name: "codex.json"}},
		models: map[string][]cpamodels.RegisteredModel{"codex.json": {{ID: "gpt-exact"}}}, modelErrors: map[string]error{},
		definitions: map[string][]cpamodels.StaticModelDefinition{}, definitionError: map[string]error{"codex": errors.New("catalog unavailable")},
	}
	result, err := NewModelSupportService(db, client).Load(context.Background(), ModelSupportRequest{IdentityIDs: []uint{identity.ID}})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !result.Complete || result.Accounts[0].CatalogStatus != "error" || result.Accounts[0].RegisteredModels[0].DefinitionStatus != "error" {
		t.Fatalf("catalog failure must qualify capability without making support partial: %+v", result)
	}
}

func TestModelDefinitionChannelIncludesPinnedGeminiInteractions(t *testing.T) {
	channel, ok := modelDefinitionChannel("gemini-interactions")
	if !ok || channel != "gemini-interactions" {
		t.Fatalf("expected exact pinned gemini-interactions channel, got %q ok=%v", channel, ok)
	}
}

func TestLoadModelSupportRejectsOversizeOrInvalidScopeBeforeUpstreamCalls(t *testing.T) {
	db := openSyncTestDatabase(t)
	client := &modelSupportClientStub{}
	service := NewModelSupportService(db, client)
	tooMany := make([]uint, ModelSupportMaxAccounts+1)
	for index := range tooMany {
		tooMany[index] = uint(index + 1)
	}
	if _, err := service.Load(context.Background(), ModelSupportRequest{IdentityIDs: tooMany}); !errors.Is(err, ErrModelSupportScopeTooLarge) {
		t.Fatalf("expected oversize error, got %v", err)
	}
	if _, err := service.Load(context.Background(), ModelSupportRequest{IdentityIDs: []uint{999}}); !errors.Is(err, ErrModelSupportScopeInvalid) {
		t.Fatalf("expected invalid identity error, got %v", err)
	}
	if client.requestCount != 0 {
		t.Fatalf("invalid scopes must not reach CPA, got %d calls", client.requestCount)
	}
}

func TestLoadModelSupportHonorsConcurrencyAndTotalTimeout(t *testing.T) {
	db := openSyncTestDatabase(t)
	identities := make([]entities.UsageIdentity, 6)
	client := &modelSupportClientStub{
		files: map[string]authfiles.AuthFile{}, models: map[string][]cpamodels.RegisteredModel{}, modelErrors: map[string]error{},
		definitions: map[string][]cpamodels.StaticModelDefinition{"codex": {}}, definitionError: map[string]error{}, delay: 5 * time.Millisecond,
	}
	for index := range identities {
		identity := &identities[index]
		identity.Name = fmt.Sprintf("Account %d", index)
		identity.AuthType = entities.UsageIdentityAuthTypeAuthFile
		identity.Identity = fmt.Sprintf("auth-%d", index)
		identity.Type = "codex"
		identity.Provider = "Codex"
		client.files[identity.Identity] = authfiles.AuthFile{AuthIndex: identity.Identity, Name: fmt.Sprintf("%d.json", index)}
		client.models[fmt.Sprintf("%d.json", index)] = []cpamodels.RegisteredModel{}
	}
	if err := db.Create(&identities).Error; err != nil {
		t.Fatalf("seed identities: %v", err)
	}
	service := NewModelSupportService(db, client)
	service.concurrency = 2
	ids := make([]uint, len(identities))
	for index := range identities {
		ids[index] = identities[index].ID
	}
	result, err := service.Load(context.Background(), ModelSupportRequest{IdentityIDs: ids})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if client.maxActiveCalls > 2 {
		t.Fatalf("expected at most two concurrent calls, saw %d", client.maxActiveCalls)
	}
	if client.maxActiveCalls != 2 {
		t.Fatalf("expected bounded worker pool to use two slots, saw %d", client.maxActiveCalls)
	}
	if !result.Complete {
		t.Fatalf("expected complete bounded load, got %+v", result)
	}

	blocked := make(chan struct{})
	timeoutClient := &modelSupportClientStub{
		files: map[string]authfiles.AuthFile{}, models: map[string][]cpamodels.RegisteredModel{}, modelErrors: map[string]error{},
		definitions: map[string][]cpamodels.StaticModelDefinition{}, definitionError: map[string]error{}, block: blocked,
	}
	timeoutService := NewModelSupportService(db, timeoutClient)
	timeoutService.timeout = 10 * time.Millisecond
	timed, err := timeoutService.Load(context.Background(), ModelSupportRequest{IdentityIDs: []uint{identities[0].ID}})
	if err != nil {
		t.Fatalf("timeout should be represented in the result, got %v", err)
	}
	if timed.Complete || timed.Accounts[0].ErrorCode != "upstream_error" {
		t.Fatalf("expected explicit timed-out partial result, got %+v", timed)
	}
}
