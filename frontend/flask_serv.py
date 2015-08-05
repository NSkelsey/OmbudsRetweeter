import requests
from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello_world():
    r = requests.get('http://localhost:8080/author/mvnrngzsNFdbHrRYqdZNC8Y6aoS9tZRMRu')
    bltns = r.json()
    return render_template(home.html, bltns)

if __name__ == '__main__':
    app.run(debug=True)
