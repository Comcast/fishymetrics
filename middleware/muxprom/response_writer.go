package muxprom

import "net/http"

type statusResponseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (s *statusResponseWriter) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusResponseWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = 200
	}
	n, err := s.ResponseWriter.Write(b)
	s.size += n
	return n, err
}
