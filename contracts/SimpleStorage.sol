pragma solidity ^0.8.19;

contract SimpleStorage {
    uint256 private storedValue;

    function set(uint256 value) public {
        storedValue = value;
    }

    function get() public view returns (uint256) {
        return storedValue;
    }

    function multiply(uint256 factor) public {
        storedValue *= factor;
    }

    function reset() public {
        storedValue = 0;
    }
}
