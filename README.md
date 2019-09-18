O servidor funciona como um proxy entre o PC e o App, com autenticacao e validacao de permissoes dos usuarios.

O usuario e senha de admin sao necessarios para registrar um novo PC e usuarios que poderao acessar o PC

Para executar:

`sudo ADMIN_USER=admin ADMIN_PASSWORD=admin docker-compose up`

As variaveis PORT e MONGODB_HOST sao opcionais

PORT padrao 9002

MONGODB_HOST padrao mongo:27017

App: https://github.com/arthurdm10/remote-pc-app

Client: https://github.com/arthurdm10/remote-pc-client
