package handler

// Ensure all response types are used (Go compiler check).
var (
	_ TenantResponse
	_ QuotaResponse
	_ TenantStatusResponse
	_ TenantTransition
	_ AgentResponse
	_ ResourceResponse
	_ ResourceSpecResponse
	_ InvoiceResponse
	_ InvoiceLineItemResponse
	_ SubscriptionResponse
	_ SecretMetadataResponse
	_ SecretResponse
	_ ModuleStatusResponse
	_ TenantListResponse
	_ AgentListResponse
	_ ResourceListResponse
	_ InvoiceListResponse
	_ SecretListResponse
)
