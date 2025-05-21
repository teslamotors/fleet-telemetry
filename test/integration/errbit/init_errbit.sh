#!/bin/bash

# Wait for Errbit to be up and running
echo "Waiting for Errbit to start..."
until $(curl --output /dev/null --silent --head --fail http://errbit:8080); do
    printf '.'
    sleep 5
done
echo "Errbit is up!"

# Run the initialization task
docker-compose exec -T errbit bundle exec rake errbit:init_errbit

echo "Errbit initialization completed!"
