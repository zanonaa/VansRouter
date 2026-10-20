package handlers

func stringField(value map[string]any, key string) string { s, _ := value[key].(string); return s }
func applyCodexClaims(data, idClaims, accessClaims map[string]any) {
	specific := map[string]any{}
	for _, claims := range []map[string]any{idClaims, accessClaims} {
		if !nodeTruthy(data["email"]) {
			if email := claims["email"]; nodeTruthy(email) {
				data["email"] = email
			} else if profile, ok := claims["https://api.openai.com/profile"].(map[string]any); ok && nodeTruthy(profile["email"]) {
				data["email"] = profile["email"]
			}
		}
		auth, _ := claims["https://api.openai.com/auth"].(map[string]any)
		for source, target := range map[string]string{"chatgpt_account_id": "chatgptAccountId", "chatgpt_plan_type": "chatgptPlanType"} {
			if !nodeTruthy(specific[target]) && nodeTruthy(auth[source]) {
				specific[target] = auth[source]
			}
		}
		for source, target := range map[string]string{"account_id": "chatgptAccountId", "plan_type": "chatgptPlanType", "chatgpt_account_id": "chatgptAccountId"} {
			if !nodeTruthy(specific[target]) && nodeTruthy(claims[source]) {
				specific[target] = claims[source]
			}
		}
	}
	if len(specific) > 0 {
		data["providerSpecificData"] = specific
	}
}
