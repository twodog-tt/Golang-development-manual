// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @dev 教学用最小 ERC20，仅供 abigen 示例部署。
contract SimpleToken {
    mapping(address => uint256) public balanceOf;

    event Transfer(address indexed from, address indexed to, uint256 value);

    constructor(uint256 initialSupply) {
        balanceOf[msg.sender] = initialSupply;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        require(balanceOf[msg.sender] >= amount, "insufficient");
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }
}
