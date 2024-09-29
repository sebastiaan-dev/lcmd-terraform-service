rm -r ./dist
mkdir -p dist
cd backend && CC=musl-gcc go build -trimpath -ldflags "-s -w -linkmode external -extldflags -static -X 'main.Version=1.1.66' -X main.build=2024-09-27_18:07:36.00a7e63df68930e8989376bca60c3de1a69c73d9" -o ../dist/td-go
cd ../ui && vite build --outDir ../dist/web
