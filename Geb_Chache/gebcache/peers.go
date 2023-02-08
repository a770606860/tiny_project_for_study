package gebcache

// 从对端获取数据
type PeerGetter interface {
	Get(group, key string) ([]byte, error)
}
