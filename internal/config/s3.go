package config

import "strconv"

const (
	defaultS3Region        = "us-east-1"
	defaultS3UsePathStyle  = true
	defaultS3UsePathStyleS = "true"

	envS3Endpoint      = "GOPHPROFILE_S3_ENDPOINT"
	envS3Region        = "GOPHPROFILE_S3_REGION"
	envS3AccessKey     = "GOPHPROFILE_S3_ACCESS_KEY"
	envS3SecretKey     = "GOPHPROFILE_S3_SECRET_KEY"
	envS3Bucket        = "GOPHPROFILE_S3_BUCKET"
	envS3UsePathStyle  = "GOPHPROFILE_S3_USE_PATH_STYLE"
)

// S3Config хранит настройки подключения к S3-совместимому хранилищу.
type S3Config struct {
	Endpoint     string
	Region       string
	AccessKey    string
	SecretKey    string
	Bucket        string
	UsePathStyle bool
}

// LoadS3 загружает настройки S3/MinIO из переменных окружения.
func LoadS3() S3Config {
	return S3Config{
		Endpoint:     getEnv(envS3Endpoint, ""),
		Region:       getEnv(envS3Region, defaultS3Region),
		AccessKey:    getEnv(envS3AccessKey, ""),
		SecretKey:    getEnv(envS3SecretKey, ""),
		Bucket:        getEnv(envS3Bucket, ""),
		UsePathStyle: parseBoolEnv(envS3UsePathStyle, defaultS3UsePathStyle),
	}
}

// parseBoolEnv читает bool-значение из переменной окружения.
func parseBoolEnv(key string, defaultValue bool) bool {
	rawValue := getEnv(key, defaultS3UsePathStyleS)

	value, err := strconv.ParseBool(rawValue)
	if err != nil {
		return defaultValue
	}

	return value
}