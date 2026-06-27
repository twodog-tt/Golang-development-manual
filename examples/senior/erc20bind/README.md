# erc20bind — abigen 完整示例（S-BC-09）

使用 **solc + abigen** 生成 Go 绑定，在 **simulated 链** 上部署、转账、监听 `Transfer` 事件。

## 运行

```bash
go test ./examples/senior/erc20bind/...
```

## 重新生成绑定

```bash
cd examples/senior/erc20bind/contract
solc --abi --bin -o build SimpleToken.sol
abigen --abi build/SimpleToken.abi --bin build/SimpleToken.bin \
  --pkg erc20bind --type SimpleToken --out ../simple_token.go
```

依赖：`solc`、`github.com/ethereum/go-ethereum/cmd/abigen`。

## 文件

| 文件 | 说明 |
|------|------|
| `contract/SimpleToken.sol` | 最小 ERC20 |
| `simple_token.go` | abigen 生成（勿手改） |
| `simple_token_test.go` | Deploy / Transfer / FilterTransfer |
