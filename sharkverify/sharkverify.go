package sharkverify

import "github.com/pquerna/otp/totp"

// 验证谷歌验证码
func VerifyCode(secret string, code string) bool {
	return totp.Validate(code, secret)
}

// 生成谷歌验证码秘钥和二维码URL
func NewSecret(issuer string, accountName string) (string, string) {
	key, _ := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
	})
	return key.Secret(), key.URL()
}

// 获取谷歌验证码二维码URL
func GetQrCodeUrl(secret string, issuer string, accountName string) (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Secret:      []byte(secret),
	})
	return key.URL(), err
}
