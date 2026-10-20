package handlers

func defaultTokenMap(tokens map[string]any) map[string]any {
	mapped := map[string]any{}
	for source, target := range map[string]string{"access_token": "accessToken", "refresh_token": "refreshToken", "id_token": "idToken"} {
		if v, ok := tokens[source]; ok {
			mapped[target] = v
		}
	}
	if v, ok := tokens["resource_url"]; ok {
		mapped["providerSpecificData"] = map[string]any{"resourceUrl": v}
	}
	return mapped
}
