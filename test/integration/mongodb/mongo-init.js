// Create the errbit db
db = db.getSiblingDB('errbit');

db.createUser({
  'user': "errbit_user",
  'pwd': "errbit_password",
  'roles': [{
    'role': 'dbOwner',
    'db': 'errbit'}]});

db.createCollection('dummy');
