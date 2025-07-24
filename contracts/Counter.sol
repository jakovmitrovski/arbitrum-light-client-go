// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

contract Counter {
    uint256 public count;

    function increment() public {
        count++;
    }

    function incrementBy(uint256 value) public {
        count += value;
    }

    function reset() public {
        count = 0;
    }
}
