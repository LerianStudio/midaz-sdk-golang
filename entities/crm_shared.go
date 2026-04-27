package entities

func crmHeaders(organizationID string) map[string]string {
	return map[string]string{"X-Organization-Id": organizationID}
}
