package providercontext

import "context"

type RuntimeData struct {
	Environment string            `json:"environment,omitempty"`
	Credentials map[string]string `json:"credentials,omitempty"`
}

type runtimeDataKey struct{}

func WithRuntimeData(ctx context.Context, data RuntimeData) context.Context {
	cloned := RuntimeData{
		Environment: data.Environment,
		Credentials: cloneStringMap(data.Credentials),
	}
	return context.WithValue(ctx, runtimeDataKey{}, cloned)
}

func RuntimeDataFromContext(ctx context.Context) (RuntimeData, bool) {
	if ctx == nil {
		return RuntimeData{}, false
	}
	value, ok := ctx.Value(runtimeDataKey{}).(RuntimeData)
	if !ok {
		return RuntimeData{}, false
	}
	value.Credentials = cloneStringMap(value.Credentials)
	return value, true
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}
