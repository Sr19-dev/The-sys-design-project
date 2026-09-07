
4 questions:
HTTP Method: To submit a long URL to your server to be shortened, should the client send a GET request or a POST request? Why?
A: to submit a long url to my server the client should send a post request as a get implies fetching from the client side, when 
we're doing the opposite
Verdict: Spot on. GET requests should be "idempotent" (meaning they don't change anything on the server; they just read data). You are creating a new record, so POST is exactly right.

Status Code: When a user clicks a short link (like localhost:8080/xyz), what specific HTTP status code (e.g., 200, 404, 500) must the server send back to physically force the browser to redirect to the original site?
A: maybe a 404 since the site is only client side and not on the internet so the site won't load
Actual answer: Verdict: Incorrect, but understandable. A 404 Not Found is a dead end. It tells the browser, "I don't have this, stop trying." To physically force a browser to navigate to a new page, HTTP uses the 3xx family of status codes—specifically 301 Moved Permanently or 302 Found (Temporary Redirect). When a server sends a 302, it attaches a Location header with the destination URL. The browser sees the 302, reads the header, and immediately routes the user to the new site.

Architecture: How would you structure the base62 encoder: counter-based (incrementing an ID like 1, 2, 3 in the database and encoding that number) or hash-based (hashing the long URL itself and taking the first 6 characters)? Why?
A: an hash based number could be better for two reasons
1. from a security perspective a number assigned via hashing is harder to crack than an incrementing ID format which after a guess becomes vulnerable 
2. in a database it might be easier to uniquely sort hashes based on the url and it's 1st 6 numbers, creating quicker look up times
Actual answer: The Spec: We will use a Random Base62 String generator. It gives you the unpredictable security you correctly identified, but avoids the collision nightmares of a truncated hash.

JSON Shape: What exactly should the raw JSON look like for the incoming request and the outgoing response when creating a short link? Write out the brackets and fields you would expect to use.
A: this is a technical question, unsure
actual answer:
{
    "url": "https://google.com"
}
{
    "short_code": "xyz789"
}