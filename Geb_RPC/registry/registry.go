package registry

type _IPv4 = string
type _name = string
type _id = int

func getHttpURL(addr _IPv4, path string) string {
	return "http://" + addr + path
}
