package util

import (
	"os"
	"strconv"
	"strings"
)

func Env(k, def string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	return v
}

func MustEnv(k string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		panic("missing env: " + k)
	}
	return v
}

// BoolEnv reads an env var as bool. If empty or invalid, returns def.
// strconv.ParseBool accepts: 1,t,T,TRUE,true,True,0,f,F,FALSE,false,False.
func BoolEnv(k string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
