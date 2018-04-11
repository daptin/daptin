# Kubernetes deployment

Daptin can be infinitely scaled on kubernetes

!!! example
    ```yaml
    apiVersion: v1
    kind: Service
    metadata:
      name: daptin-instance
      labels:
        app: daptin
    spec:
      ports:
        - port: 8080
      selector:
        app: daptin
        tier: production
    ---
    apiVersion: extensions/v1beta1
    kind: Deployment
    metadata:
      name: daptin-daptin
      labels:
        app: daptin
    spec:
      strategy:
        type: Recreate
      template:
        metadata:
          labels:
            app: daptin
            tier: testing
        spec:
          containers:
          - image: daptin/daptin:latest
            name: daptin
            args: ['-db_type', 'mysql', '-db_connection_string', 'user:password@tcp(<mysql_service>:3306)/daptin']
            ports:
            - containerPort: 8080
              name: daptin
    ---
    apiVersion: extensions/v1beta1
    kind: Ingress
    metadata:
      name: daptin-test
    spec:
      rules:
      - host: hello.website
        http:
          paths:
          - backend:
              serviceName: daptin-testing
              servicePort: 8080
    ```
