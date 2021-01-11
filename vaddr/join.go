package vaddr

func Join(s ...Suite) Suite {
	var r Suite
	for _, c := range s {
		r.Wrappers = append(r.Wrappers, c.Wrappers...)
		r.Actives = append(r.Actives, c.Actives...)
	}
	return r
}
