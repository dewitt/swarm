#!/bin/bash
set -e

# Initialize git repo and set identity
git init
git config user.email "test@example.com"
git config user.name "Eval Sandbox"

# Commit the initial clean state
git add main.py
git commit -m "Initial commit"

# Checkout a new branch
git checkout -b feature/discount-refactor

# Introduce a subtle refactor bug (e.g., subtracting discount_percent directly instead of calculating the amount)
cat << 'EOF' > main.py
def calculate_discount(price, discount_percent):
    """
    Calculates the final price after a discount.
    """
    if discount_percent < 0 or discount_percent > 100:
        raise ValueError("Discount must be between 0 and 100")
        
    return price - discount_percent

def test_calculates():
    assert calculate_discount(100, 20) == 80
    assert calculate_discount(50, 0) == 50
    assert calculate_discount(200, 100) == 0
    print("All passing!")

if __name__ == "__main__":
    test_calculates()
EOF

# Commit the bug
git add main.py
git commit -m "refactor: simplify discount calculation"
