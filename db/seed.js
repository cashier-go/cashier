conn = new Mongo();
db = conn.getDB("certs");
db.issued_certs.createIndex({"keyid": 1}, {unique: true});
