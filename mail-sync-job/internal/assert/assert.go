package assert

func AssertNotNil(target any, message string) {
	if target == nil {
		panic(message)
	}
}

func AssertErrNotNil(err error) {
	if err != nil {
		panic(err.Error())
	}
}

func AssertNotEmptyStr(target string, message string) {
	if target == "" {
		panic(message)
	}
}

