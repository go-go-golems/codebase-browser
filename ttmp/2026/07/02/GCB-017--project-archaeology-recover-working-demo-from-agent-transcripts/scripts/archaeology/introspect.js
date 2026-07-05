function tables() {
  const mt = require('minitrace');
  const db = mt.db().RuntimeArchives().QueryCommandDefaults().Build();
  try {
    return db.tables();
  } finally { db.close(); }
}

function columns() {
  const mt = require('minitrace');
  const db = mt.db().RuntimeArchives().QueryCommandDefaults().Build();
  try {
    return db.schema();
  } finally { db.close(); }
}

__verb__('tables', { name: 'tables', short: 'List normalized SQLite tables' });
__verb__('columns', { name: 'columns', short: 'List normalized SQLite columns' });
