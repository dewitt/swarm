import random
import statistics
from multiprocessing import Pool


def generate_random_number(_):
    return random.randint(1, 10)


if __name__ == "__main__":
    with Pool(processes=10) as pool:
        numbers = pool.map(generate_random_number, range(10))
    print(statistics.median(numbers))
