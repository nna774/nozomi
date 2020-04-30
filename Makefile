all: app

app:
	go build

deploy: app
	~/.local/bin/sam package --template-file template.yml --s3-bucket nana-lambda --output-template-file packaged-template.yml
	~/.local/bin/sam deploy --template-file packaged-template.yml --region ap-northeast-1 --stack-name nozomi
