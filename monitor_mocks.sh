# Generate mocks for Clipboard interface
mockgen -source=internal/clipboard/clipboard.go -destination=internal/clipboard/mock_clipboard.go -package=clipboard

# Generate mocks for Storage interface
mockgen -source=internal/storage/storage.go -destination=internal/storage/mock_storage.go -package=storage

# Generate mocks for MQTTClient interface
mockgen -source=internal/broker/mqtt.go -destination=internal/broker/mock_mqtt.go -package=broker

#test cmds
go test -v ./internal/clipboard
go test -bench=. ./internal/clipboard


# To make these tests even more robust, you could consider:

# Adding more test cases to the table-driven tests.
# Implementing property-based testing using a library like gopter.
# Adding integration tests that test the Monitor with real (non-mock) dependencies.
# Using a coverage tool to ensure you're testing all parts of your code:

go test -coverprofile=coverage.out ./internal/clipboard
go tool cover -html=coverage.out

# This will generate a coverage report and open it in your browser, showing which parts of your code are covered by tests.
