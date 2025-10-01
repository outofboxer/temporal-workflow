package temporal

import "go.temporal.io/sdk/client"

// This is custom struct wrapping the official client.
type Client struct {
	client client.Client
}

// NewClient initializes the connection to the Temporal frontend.
func NewClient(hostPort, namespace string) (client.Client, error) {
	// In a real app, you'd get the host from Encore's config/secrets system.
	c, err := client.Dial(client.Options{
		HostPort:  hostPort,
		Namespace: namespace,
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (tc *Client) Close() {
	tc.client.Close()
}
