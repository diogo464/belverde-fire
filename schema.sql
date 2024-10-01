CREATE TABLE IF NOT EXISTS location (
	timestamp 	REAL NOT NULL,
	latitude 	REAL NOT NULL,
	longitude 	REAL NOT NULL,
	accuracy	REAL NOT NULL,	
	heading		REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS shape (
	id			INTEGER NOT NULL,
	kind		VARCHAR NOT NULL
);

CREATE TABLE IF NOT EXISTS shape_point (
	shape		INTEGER NOT NULL,
	latitude	REAL NOT NULL,
	longitude	REAL NOT NULL
);
