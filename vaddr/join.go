package vaddr

func Join(s ...Suite) Suite {
	var r Suite
	for _, c := range s {
		w, a := Split(c)
		r.Wrappers = append(r.Wrappers, w)
		r.Actives = append(r.Actives, a)
	}
	return r
}
