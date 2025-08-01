package codec

import "go.uber.org/zap"

type ConsoleReader struct {
	lines  chan string
	close  func()
	done   chan any
	logger *zap.Logger
}
