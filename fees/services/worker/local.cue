package worker

// The DSN secret is loaded from the .secrets file, so this section is empty.

if (#Meta.Environment.Type == "development" || #Meta.Environment.Type == "test") && #Meta.Environment.Cloud == "local" {
  #Config: {
    Temporal: {
      Address:   "127.0.0.1:7233"
      Namespace: "default"
      UseTLS:    false
      UseAPIKey: false
      Host: "localhost:7233"
    }
  }
}
#Config