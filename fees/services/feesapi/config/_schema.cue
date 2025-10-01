package feesapi

#Config: {
  DB: {
    MaxOpenConns: *10 | int
    MaxIdleConns: *5  | int
  }
  Temporal: {
    Address:   *"127.0.0.1:7233" | string
    Namespace: *"default"        | string
    UseTLS:    *false            | bool
    UseAPIKey: *false            | bool
  }
}
#Config