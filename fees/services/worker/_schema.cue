package worker

#Config: {
  Temporal: {
    Address:   *"127.0.0.1:7233" | string
    Namespace: *"default"        | string
    UseTLS:    *false            | bool
    UseAPIKey: *false            | bool
  }
}
#Config