package controllers

func OwnedByKrok(annotations map[string]string) bool {
	_, ok := annotations[krokAnnotationKey]
	return ok
}
