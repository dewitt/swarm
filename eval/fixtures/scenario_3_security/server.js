const express = require('express');
const sqlite3 = require('sqlite3');
const jwt = require('jsonwebtoken');

const app = express();
app.use(express.json());

const db = new sqlite3.Database(':memory:');
db.serialize(() => {
    db.run("CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, password TEXT)");
    db.run("INSERT INTO users (username, password) VALUES ('admin', 'supersecret')");
});

const JWT_SECRET = "my_super_secret_hardcoded_jwt_key_12345";

app.post('/login', (req, res) => {
    const { username, password } = req.body;

    // Blatant SQL Injection vulnerability
    const query = `SELECT * FROM users WHERE username = '${username}' AND password = '${password}'`;

    db.get(query, (err, row) => {
        if (err) {
            return res.status(500).json({ error: err.message });
        }
        if (!row) {
            return res.status(401).json({ error: "Invalid credentials" });
        }

        const token = jwt.sign({ id: row.id, username: row.username }, JWT_SECRET, { expiresIn: '1h' });
        res.json({ token });
    });
});

app.listen(3000, () => {
    console.log('Server is running on port 3000');
});
