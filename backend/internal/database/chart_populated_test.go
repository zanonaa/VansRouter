package database

import (
	"context"
	"testing"
	"time"
)

func TestHourlyChartPopulatedAndNulls(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	timestamp := time.Now().UTC().Add(-time.Minute).Format("2006-01-02T15:04:05.000Z")
	if _, err = db.Exec(`INSERT INTO usageHistory(timestamp,promptTokens,completionTokens,cost) VALUES(?,12,8,0.25),(?,NULL,NULL,NULL)`, timestamp, timestamp); err != nil {
		t.Fatal(err)
	}
	for _, period := range []string{"24h", "today"} {
		series, err := (Usage{DB: db}).Chart(ctx, period)
		if err != nil {
			t.Fatal(err)
		}
		if len(series) != 24 {
			t.Fatal(len(series))
		}
		var requests, tokens int64
		var cost float64
		for _, bucket := range series {
			requests += bucket["requests"].(int64)
			tokens += bucket["tokens"].(int64)
			cost += bucket["cost"].(float64)
		}
		// today pode excluir o registro se o teste rodar no primeiro minuto do dia.
		if period == "24h" && (requests != 2 || tokens != 20 || cost != 0.25) {
			t.Fatalf("%s: %d %d %f", period, requests, tokens, cost)
		}
	}
}
