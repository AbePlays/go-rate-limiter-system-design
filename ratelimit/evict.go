package ratelimit

const defaultMaxKeys = 10000

func evictIfFull[V any](m map[string]V, maxKeys int, stale func(V) bool) {
	if len(m) < maxKeys {
		return
	}
	for k, v := range m {
		if stale(v) {
			delete(m, k)
		}
	}
	if len(m) < maxKeys {
		return
	}
	for k := range m {
		delete(m, k)
		break
	}
}
