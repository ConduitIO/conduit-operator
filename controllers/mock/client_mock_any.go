package mock

import "github.com/golang/mock/gomock"

func ClientOptions() gomock.Matcher {
	return gomock.Any()
}
