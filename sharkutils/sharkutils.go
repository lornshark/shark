package sharkutils

import (
	"crypto/md5"
	"encoding/hex"
	crand "math/rand/v2"
	"net"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// RandNum 生成[min,max)之间的随机数
func RandNum(min int, max int) int {
	if min >= max {
		return min
	}
	return crand.IntN(max-min) + min
}

// RandString 生成指定长度的随机字符串
func Md5(data []byte) string {
	h := md5.New()
	return hex.EncodeToString(h.Sum(data))
}

// GetClientIp 获取客户端 IP 地址
func GetClientIp(request *http.Request) string {
	ip := request.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = request.Header.Get("X-Real-Ip")
	}
	if ip == "" {
		ip, _, _ = net.SplitHostPort(request.RemoteAddr)
	} else {
		ip = strings.Split(ip, ",")[0]
	}
	return ip
}

// bcrypt加密密码
func BcryptHash(password string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes)
}

// bcrypt验证密码 password: 明文密码 bcryptHash: bcrypt加密后的密码
func BcryptCheck(password string, bcryptHash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(bcryptHash), []byte(password))
	return err == nil
}
