package api

import (
	"strings"

	"cpa-usage/internal/entities"
)

type usageIdentityResolver struct {
	authFilesByIdentity map[string]entities.UsageIdentity
	providersByIdentity map[string]entities.UsageIdentity
}

func newUsageIdentityResolver(identities []entities.UsageIdentity) usageIdentityResolver {
	authFilesByIdentity := make(map[string]entities.UsageIdentity, len(identities))
	providersByIdentity := make(map[string]entities.UsageIdentity, len(identities))
	for _, identity := range identities {
		if identity.IsDeleted {
			continue
		}
		key := strings.TrimSpace(identity.Identity)
		if key == "" {
			continue
		}
		authTypeName, ok := identity.AuthType.CanonicalName()
		if !ok {
			continue
		}
		switch authTypeName {
		case entities.UsageIdentityAuthTypeNameOAuth:
			authFilesByIdentity[key] = identity
		case entities.UsageIdentityAuthTypeNameAPIKey:
			providersByIdentity[key] = identity
		}
	}

	return usageIdentityResolver{
		authFilesByIdentity: authFilesByIdentity,
		providersByIdentity: providersByIdentity,
	}
}

type resolvedUsageIdentity struct {
	DisplayName string
	Type        string
}

func resolvedUsageIdentityFromEntity(item entities.UsageIdentity) resolvedUsageIdentity {
	return resolvedUsageIdentity{
		DisplayName: item.DisplayName(),
		Type:        strings.TrimSpace(item.Type),
	}
}

func (r usageIdentityResolver) resolveByAuthIndex(authIndex string) (resolvedUsageIdentity, bool) {
	key := strings.TrimSpace(authIndex)
	if key == "" {
		return resolvedUsageIdentity{}, false
	}
	if identity, ok := r.providersByIdentity[key]; ok {
		return resolvedUsageIdentityFromEntity(identity), true
	}
	if identity, ok := r.authFilesByIdentity[key]; ok {
		return resolvedUsageIdentityFromEntity(identity), true
	}
	return resolvedUsageIdentity{}, false
}
