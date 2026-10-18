package sqlite

import "encoding/json"

type reference struct{ bucket, id string }

func references(bucket string, raw []byte) []reference {
	var v map[string]json.RawMessage
	_ = json.Unmarshal(raw, &v)
	result := []reference{}
	add := func(field, b string) {
		var id string
		_ = json.Unmarshal(v[field], &id)
		if id != "" {
			result = append(result, reference{b, id})
		}
	}
	switch bucket {
	case "servers":
		var spec map[string]json.RawMessage
		_ = json.Unmarshal(v["spec"], &spec)
		var id string
		_ = json.Unmarshal(spec["router_id"], &id)
		if id != "" {
			result = append(result, reference{"routers", id})
		}
	case "users":
		add("router_id", "routers")
		add("server_id", "servers")
		add("account_id", "accounts")
	case "jobs":
		add("router_id", "routers")
	case "sessions":
		add("AccountID", "accounts")
	case "deliveries":
		add("AccountID", "accounts")
		add("UserID", "users")
	}
	return result
}
