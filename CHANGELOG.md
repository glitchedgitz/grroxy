## Current Changes

- ReadFile param changes from `from` to `location` changed to

```javascript
    //previously 
    const filedata = await readFile({
        fileName: filename,
        from: 'cwd'
    });

    //now 
    const filedata = await readFile({
        fileName: filename,
        folder: 'cwd'
    });
```