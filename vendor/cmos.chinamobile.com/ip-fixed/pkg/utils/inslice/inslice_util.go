package inslice

func InStringSlice(slice []string, key string) bool {
	for _, v := range slice {
		if v == key {
			return true
		}
	}

	return false
}

func InStringSliceMapKeyFunc(slice []string) func(string) bool {
	set := make(map[string]struct{})

	for _, v := range slice {
		set[v] = struct{}{}
	}

	return func(key string) bool {
		_, ok := set[key]
		return ok
	}
}

func InIntSlice(slice []int, key int) bool {
	for _, v := range slice {
		if v == key {
			return true
		}
	}

	return false
}

func InIntSliceMapKeyFunc(slice []int) func(int) bool {
	set := make(map[int]struct{})

	for _, v := range slice {
		set[v] = struct{}{}
	}

	return func(key int) bool {
		_, ok := set[key]
		return ok
	}
}
