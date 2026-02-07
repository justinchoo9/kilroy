package engine

func cxdbContextID(s *CXDBSink) string {
	if s == nil {
		return ""
	}
	return s.ContextID
}
