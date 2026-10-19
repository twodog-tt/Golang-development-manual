// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

interface PaymentToken {
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}

// Local fixture: a reference binds each deposit to one invoice. This is not
// a production checkout contract: invoice registration/expiry is offchain.
contract PaymentReceiver {
    PaymentToken public immutable token;
    address public immutable custody;
    mapping(bytes32 => bool) public paid;
    event Paid(bytes32 indexed invoice, address indexed payer, uint256 amount);
    constructor(address t, address c) { token = PaymentToken(t); custody = c; }
    function pay(bytes32 invoice, uint256 amount) external {
        require(!paid[invoice] && amount > 0, "duplicate/amount");
        paid[invoice] = true;
        require(token.transferFrom(msg.sender, custody, amount), "transfer");
        emit Paid(invoice, msg.sender, amount);
    }
}
