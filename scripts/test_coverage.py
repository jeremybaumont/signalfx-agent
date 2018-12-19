
import argparse
from functools import reduce
import json
import os
import pprint
import pytest
import re
import sys
from tabulate import tabulate
import time

DEBUG=False
DESCRIPTION = "signalfx-agent test coverage reporter"
FEATURES = [ 'monitors', 'observers' ]
FEATURETYPES = [ 'monitorType', 'observerType' ]
LS = "\n"
MISC_FEATURES = ['basic', 'config_sources', 'packaging']
REPORT_DATA = {
    'coverage' : {
        'headers' : [ 'Feature', 'Percentage (%)' ],
        'msg' : LS + "Coverage status:" + LS,
        'report' : [],
    },
    'missingtests' : {
        'headers' : [ 'Monitors', 'Observers' ],
        'msg' : LS + "Features do not have testcases:" + LS,
        'report' : [],
    },
    'miscellaneous' : {
        'headers' : [ 'Basic', 'ConfigSources', 'Packaging' ],
        'msg' : LS + "Miscellaneous testcases available:" + LS,
        'report' : [],
    },
}
substitutes = { '_' : '-', '-collectd' : '',  'collectd-' : '', 'prometheus' : 'prometheus-exporter' }
re_node_name = re.compile(r'^%s(.*)' % 'test_')
re_module_name = re.compile(r'(.*)%s$' % '_test')
re_tests_sub = re.compile('|'.join(map(re.escape, substitutes.keys())))

class MyPlugin:
    def __init__(self):
        self.collected = []
    def pytest_collection_modifyitems(self, items):
        for item in items:
            self.collected.append(item)

class ParseArgs:
    def __init__(self):
        pass

    def valid_file(self, param):
        base, ext = os.path.splitext(param)
        if ext.lower() not in ('.json'):
            raise argparse.ArgumentTypeError('Given File must have a json extension')
        return param

    def valid_dir(self, param):
        if not os.path.exists(param):
            raise argparse.ArgumentTypeError('Given Tests directory does not exists')
        return param

    def parse_args(self):
        parser = argparse.ArgumentParser(
            formatter_class=argparse.RawDescriptionHelpFormatter,
            description=DESCRIPTION)
        parser.add_argument('-f', '--file', help='signalfx-agent self describe file input',
                            type=self.valid_file, required=True)
        parser.add_argument('-t', '--tests-dir', help='signalfx-agent pytest testcases directory',
                            type=self.valid_dir, required=True)
        parser.add_argument('-d', '--debug', help='Debug mode',
                            type=bool, required=False, default=DEBUG)
        return parser.parse_args()

def read_validate_json(file):
    print("Processing self describe data")
    with open(file) as f:
        data = json.load(f)
    if type(data) == dict:
        status = reduce(
            (lambda x, y: x * y), 
            [ True if key.title() in data.keys() else False for key in FEATURES ]
        )
        if not status:
            data = False
    else:
        data = False
    if not data:
        print("Invalid file input, exiting.")
        sys.exit()
    return data

def get_types(data):
    types = { 
        type : [ 
            v.replace("collectd/", "") 
            for element in data[type.title()] for k, v in element.items() if k in FEATURETYPES 
        ] 
        for type in FEATURES
    }
    if DEBUG:
        print("Types:")
        pprint.pprint(types)
    return types

def get_node_details_tests(nodeid):
    tmppkg = nodeid.parent.name.split("tests/")
    index = 1 if len(tmppkg) > 1 else 0
    package = tmppkg[index].split('/')[0]
    modulename = nodeid.module.__name__
    tmpmod = modulename.split('.')
    module = tmpmod[1] if len(tmpmod) > 1 else tmpmod[0]
    name = re.sub(re_node_name, r'\1', nodeid.name)
    module = re.sub(re_module_name, r'\1', module)
    return (package, module, name)

def get_node_details(nodeid):
    name = nodeid.name
    modulename = nodeid.module.__name__
    tmp = modulename.split('.')
    if len(tmp) > 1:
        package, module = tmp[0:2]
    else:
        package, module = 'flaky', tmp[0]
    name = re.sub(re_node_name, r'\1', name)
    module = re.sub(re_module_name, r'\1', module)
    return (package, module, name)

def fetch_process_pytests(tests_dir):
    print("Collecting and procesing pytests data")
    my_plugin = MyPlugin()
    pytest.main(['--collect-only', '-p', 'no:terminal', tests_dir], plugins=[my_plugin])
    testsdata = dict()
    for nodeid in my_plugin.collected:
        package, module, name = get_node_details_tests(nodeid)
        if not package in testsdata:
            testsdata[package] = dict()
        if not module in testsdata[package]:
            testsdata[package][module] = dict()
            testsdata[package][module]["general"] = list()
        if '[' in name:
            key, value = name.split('[')
            if key in testsdata[package][module]:
                testsdata[package][module][key].append(value.strip(']'))
            else:
                testsdata[package][module][key] = [ value.strip(']') ]
        else:
            testsdata[package][module]["general"].append(name)
    status = reduce(
        (lambda x, y: x * y), 
        [ True if key in testsdata.keys() else False for key in FEATURES + MISC_FEATURES ]
    )
    if not status:
        print("Invalid tests location input is observed, exiting.")
        sys.exit()
    if DEBUG:
        print("Tests:")
        pprint.pprint(testsdata)
    return testsdata

def print_report():
    msg = "Test coverage report for signalfx-agent"
    for k in REPORT_DATA.keys():
        msg += REPORT_DATA[k]['msg']
        msg += tabulate(REPORT_DATA[k]['report'], headers=REPORT_DATA[k]['headers'], tablefmt="fancy_grid")
    print(msg)

def generate_report(types, tests):
    global REPORT_DATA
    diff_mon = list(
        set([x.replace('_', '-') for x in types['monitors']])
         - 
        set([re_tests_sub.sub(lambda m: substitutes[m.group(0)], x) for x in tests['monitors'].keys()])
        )
    diff_obs = list(set(types['observers']) - set(tests['observers'].keys()))
    for type in FEATURES:
        percentage = len(tests[type]) * 100 / len(types[type])
        REPORT_DATA['coverage']['report'].append((type.title(), percentage))
    diff_mon = { k : v for k, v in enumerate(diff_mon) }
    diff_obs = { k : v for k, v in enumerate(diff_obs) }
    for i in range(max([len(diff_mon), len(diff_obs)])):
        REPORT_DATA['missingtests']['report'].append((diff_mon.get(i, ''), diff_obs.get(i, '')))
    misc_tests = { kk : { k : v for k, v in enumerate(tests[kk].keys()) } for kk in MISC_FEATURES }
    max_value = max([len(v) for v in misc_tests.values()])
    for i in range(max_value):
        REPORT_DATA['miscellaneous']['report'].append([ misc_tests[k].get(i, '') for k in MISC_FEATURES ])
    print_report()

def main():
    global DEBUG
    start = time.time()
    args_cls = ParseArgs()
    args = args_cls.parse_args()
    json_file = args.file
    tests_dir = args.tests_dir
    DEBUG = args.debug

    data = read_validate_json(json_file)
    types = get_types(data)
    tests = fetch_process_pytests(tests_dir)
    generate_report(types, tests)
    print('Total time taken: %f minutes' % ((time.time()-start)/60))

if __name__ == '__main__':
    main()

