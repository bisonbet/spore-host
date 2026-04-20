.PHONY: build clean deploy test

build:
	@echo "Building prism-bot Lambda..."
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap .
	zip function.zip bootstrap
	@echo "Build complete: function.zip"

clean:
	rm -f bootstrap function.zip

test:
	go test ./... -v

deploy: build
	@echo "Deploying to Lambda..."
	aws lambda update-function-code \
		--function-name prism-bot \
		--zip-file fileb://function.zip
	@echo "Deploy complete"
