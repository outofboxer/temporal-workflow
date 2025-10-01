package feesapi

// The DSN secret is loaded from the .secrets file, so this section is empty.

if (#Meta.Environment.Type == "development" || #Meta.Environment.Type == "test") && #Meta.Environment.Cloud == "local" {
  #Config: {
    DB: {
      MaxOpenConns: 5
      MaxIdleConns: 2
    }
    Temporal: {
			Host: "localhost:7233"
      Namespace: "default"
      UseTLS:    false
      UseAPIKey: false
    }
  }
}
#Config