function reqListener() {
  var recs = JSON.parse(this.responseText);
  var table = document.querySelector('#cert-table');
  var tbody = table.querySelector("#list");
  while (tbody.rows.length > 0) {
    tbody.deleteRow(0);
  }
  issuedList.clear();
  recs.forEach(function makeTable(el, i, arr) {
    var row = tbody.insertRow(-1);
    row.insertCell(0).innerHTML = el.key_id;
    row.insertCell(1).innerHTML = el.created_at;
    row.insertCell(2).innerHTML = el.expires;
    row.insertCell(3).innerHTML = el.principals;
    row.insertCell(4).innerHTML = el.message;
    row.insertCell(5).innerHTML = el.revoked;
    // Index keyid and principals.
    row.cells[0].classList = ["keyid"];
    row.cells[3].classList = ["principals"];
    row.insertCell(5)
    if (!el.revoked) {
      row.cells[5].innerHTML = '<input style="margin:0;" type="checkbox" value="'+ el.key_id + '" name="cert_id" id="cert_id" />';
    }
    tbody.appendChild(row);
  });
  issuedList.reIndex();
}

function loadCerts(all) {
  var r = new XMLHttpRequest();
  var endpoint = '/admin/certs.json';
  if (all) {
    endpoint += '?all=true';
  }
  r.open('GET', endpoint);
  r.addEventListener('load', reqListener);
  r.send()
}

var SHOW_ALL = false;

function toggleExpired() {
  var button = document.querySelector("#toggle-certs");
  SHOW_ALL = !SHOW_ALL;
  loadCerts(SHOW_ALL);
  if (SHOW_ALL == false) {
    button.innerHTML = "Show Expired";
  } else {
    button.innerHTML = "Hide Expired";
  }
}
