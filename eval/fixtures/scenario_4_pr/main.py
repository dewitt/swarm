def calculate_discount(price, discount_percent):
    """
    Calculates the final price after a discount.
    """
    if discount_percent < 0 or discount_percent > 100:
        raise ValueError("Discount must be between 0 and 100")
        
    discount_amount = price * (discount_percent / 100)
    return price - discount_amount

def test_calculates():
    assert calculate_discount(100, 20) == 80
    assert calculate_discount(50, 0) == 50
    assert calculate_discount(200, 100) == 0
    print("All passing!")

if __name__ == "__main__":
    test_calculates()
