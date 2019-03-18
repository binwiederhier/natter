#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <pthread.h>
#include <natter.h>

void *broker_listen(void *vargp)
{
    natter_broker_listen((char*) ":10000");
    return NULL;
}

void *client_listen(void *vargp)
{
    natter_client_listen((char*) "bob", (char*) "localhost:10000");
    return NULL;
}

void *client_forward(void *vargp)
{
    natter_client_forward((char*) "alice", (char*) "localhost:10000", (char*) ":9000", (char*) "bob", (char*) ":22", 0, NULL);
    return NULL;
}

int main()
{
    pthread_t broker;
    pthread_t alice;
    pthread_t bob;

    pthread_create(&broker, NULL, broker_listen, NULL);
    pthread_create(&alice, NULL, client_forward, NULL);
    pthread_create(&bob, NULL, client_listen, NULL);

    pthread_join(broker, NULL);
    exit(0);
}
