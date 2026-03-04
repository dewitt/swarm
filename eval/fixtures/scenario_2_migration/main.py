import requests

def fetch_data(urls):
    results = []
    for url in urls:
        print(f"Fetching {url}...")
        response = requests.get(url)
        if response.status_code == 200:
            results.append(response.text[:50])
        else:
            results.append(f"Error: {response.status_code}")
    return results

if __name__ == "__main__":
    test_urls = [
        "https://httpbin.org/get",
        "https://httpbin.org/delay/1", 
        "https://httpbin.org/status/404"
    ]
    print(fetch_data(test_urls))
