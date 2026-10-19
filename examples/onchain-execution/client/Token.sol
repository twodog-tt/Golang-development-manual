// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

// Synthetic localnet asset. No fees, rebasing, hooks, permit or admin functions.
contract Token {
    string public name;
    string public symbol;
    uint8 public immutable decimals;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    constructor(string memory n, uint8 d, uint256 supply) {
        name = n; symbol = n; decimals = d;
        totalSupply = supply; balanceOf[msg.sender] = supply;
        emit Transfer(address(0), msg.sender, supply);
    }
    function approve(address to, uint256 value) external returns (bool) {
        allowance[msg.sender][to] = value;
        emit Approval(msg.sender, to, value); return true;
    }
    function transfer(address to, uint256 value) external returns (bool) {
        move(msg.sender, to, value); return true;
    }
    function transferFrom(address from, address to, uint256 value) external returns (bool) {
        require(allowance[from][msg.sender] >= value, "allowance");
        allowance[from][msg.sender] -= value;
        move(from, to, value); return true;
    }
    function move(address from, address to, uint256 value) internal {
        require(to != address(0) && balanceOf[from] >= value, "balance/address");
        balanceOf[from] -= value; balanceOf[to] += value;
        emit Transfer(from, to, value);
    }
}
