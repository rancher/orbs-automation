package header

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
)

var headerBytes = []byte("H4sIAAAAAAAA/8xZTc7jOgzb5xTfcb21DfSAPclD+xqGkShHzg8wAy5SRaIo2nHa+ZblL//v/apftD26Qvtm1jS9Io64+/tV9uj5VpONfK8TTed7ArJzeb/acq1xsn1/qP0hCl2ctn7dsKdVyP641XI+BP2nVIzdmbFjICGpyHjRpoUMDQFkb6AoxXz9EbRc3iFtvSV7IPI/CS+MV5M1JpJS1DWwDb1GcF3WhE3QlJRIE9Dd0CbYlT5kXjXmCqCGg8eCZhwxaEePtGHIP1iBKBPkJuMIA3FsrN9SLifXzQAPGZOzAYBRxR857fCBS0uL6CN0okUtkH+JXlaHhKLuotCgZLdcTp9R2VUQESm3xRITNiaWObrLLT2MXJnDKntGqBQ3eEMxpAKTEJXshGZUHso6lNuOxEXomXVPKGT8zn11K1KINedgBPUgfftVK9VAsgG8eDLBmzYl2+Cjaid3P9Ua1oUxmirBFvIblL/6+elkwgmYZ0N8VRmKH0zN03nU4VDM04ORx65m4D3fvwwSDVjoFJiERXhaTuAFQ1qLmZMYuJBmZQkDyFqMhTMHBxHX4qsQJHOVBGrl3YwZdAJPEqG3h8yP9iWG3Bj+hnlJbHQrpBTwi/2xr5IMjKqaZhzxw36WxZ7TK1G5YIpBTTjSnS/7Os/hUdUciMuSyBm84ZkNkX6fNx5MjhblEXswkMyP7OkBFdgqZT5tD1ogwk8VRhvPKGfh8rw56BVRYfvcYo5UyLSbUcqWAYkfgYMyn9HNf5OsVUxYXZxLJO11wHz7Rc1Ninn5I5sgR2gqn8Ez8g+JqmgBLpG0t9uyP2nkcD4i50CwqXwGT9kDNoYsuR3+vPlNuqzuyLIIcpIBuJZHBXwyrjPuRLVJdGXQr514Ux0CciRkCcDNgRqPyGmSUMKzDZR4VwD5pkq2BhCU+R4sAIMbBnP3UQgx9mdTHs19TIIlAFhbE7wRNRC8BflnU2YkyQhrZImEtIPjsuoJ2L/B0FMj8z2iXTLrideDiMx8Aua4Bcx5ImslvC2AzD8EnDnN0C4sijTnxJ7x5nAEH/999NicwW8CDNtU3DhjXBpUtRzh08AWFZ68X/39qv8NAPGSepztJAAA")

// headerFromBytes decodes the art: base64 over gzip over the text.
func headerFromBytes(encoded []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return "", err
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer func() { _ = zr.Close() }()
	art, err := io.ReadAll(zr)
	if err != nil {
		return "", err
	}
	return string(art), nil
}

// Print returns the header art, or "" if it cannot be decoded.
func Print() string {
	art, err := headerFromBytes(headerBytes)
	if err != nil {
		return ""
	}

	return art
}
