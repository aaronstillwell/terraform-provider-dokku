package dokku

import (
	"math/rand"
	"strings"
)

//
func interfaceSliceToStrSlice(list []interface{}) []string {
	slice := make([]string, len(list))

	for _, d := range list {
		slice = append(slice, d.(string))
	}
	return slice
}

//
func mapOfInterfacesToMapOfStrings(m map[string]interface{}) map[string]string {
	newMap := make(map[string]string, len(m))

	for k, v := range m {
		newMap[k] = v.(string)
	}

	return newMap
}

// Calculate which keys are in map2 which are not in map1
func calculateMissingKeys(map1 map[string]string, map2 map[string]string) []string {
	keys := make([]string, 0)

	for k := range map2 {
		if _, ok := map1[k]; !ok {
			keys = append(keys, k)
		}
	}

	return keys
}

// Calculate which strings are in slice2 but not in slice1
func calculateMissingStrings(slice1 []string, slice2 []string) []string {
	slice1Map := make(map[string]struct{})
	missing := make([]string, 0)

	for _, v := range slice1 {
		slice1Map[v] = struct{}{}
	}

	for _, v := range slice2 {
		if _, ok := slice1Map[v]; !ok {
			missing = append(missing, v)
		}
	}

	return missing
}

//
func sliceToLookupMap(slice []string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, str := range slice {
		m[str] = struct{}{}
	}
	return m
}

//
func dockerImageAndVersion(str string) (string, string) {
	parts := strings.Split(str, ":")
	return parts[0], parts[1]
}

//
func tmpResourceName(length int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, length)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
