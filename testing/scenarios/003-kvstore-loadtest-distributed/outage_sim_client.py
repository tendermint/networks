#!/usr/bin/env python3

import os
import sys
import time
import requests


OUTAGE_SIM = os.environ.get('OUTAGE_SIM', '')


class SimulationInstruction:
    def __init__(self, host, cmd):
        self.host = host
        self.cmd = cmd
    
    def execute(self):
        print("Bringing %s %s" % (self.host, self.cmd))
        r = requests.post('http://%s:34000', data=self.cmd)
        if r.status_code >= 400:
            print("FAILED: Response code %d" % r.status_code)
        else:
            print("Success!")
        print("")
    
    def __repr__(self):
        return "SimulationInstruction(host=%s, cmd=%s)" % (self.host, self.cmd)


class SimulationStep:
    def __init__(self, wait_period, instructions):
        self.wait_period = wait_period
        # we only want relevant instructions
        self.instructions = instructions
    
    def execute(self):
        if self.wait_period > 0:
            print("Waiting %d second(s)..." % self.wait_period)
            time.sleep(self.wait_period)

        for i in self.instructions:
            i.execute()
    
    def __repr__(self):
        return "SimulationStep(wait_period=%d, instructions=%s)" % (self.wait_period, self.instructions)


def extract_sim_plan():
    """Extracts the outage simulation plan from the OUTAGE_SIMULATION variable.
    
    The OUTAGE_SIMULATION structure looks as follows:

        60:tik0=up,tik1=down|360:tik0=down|60:tik0=up,tik1=up
    
    The above simulation will:
        1. Wait 60 seconds, then ensure tik0 is up and tik1 is down
        2. Then, wait 360 seconds, and ensure tik0 is down
        3. Then, wait 60 more seconds and ensure tik0 and tik1 are both up
    """
    plan = []
    steps = OUTAGE_SIM.split("|")
    for step in steps:
        s_parts = step.split(":")
        if len(s_parts) == 2:
            wait_period, instructions = int(s_parts[0]), s_parts[1].split(',')
            _instructions = []
            for i in instructions:
                i_parts = i.split('=')
                if len(i_parts) == 2:
                    _instructions.append(SimulationInstruction(i_parts[0], i_parts[1]))
            plan.append(SimulationStep(wait_period, _instructions))
    
    return plan


def main():
    sim_plan = extract_sim_plan()
    print("Built simulation plan: %d step(s)" % len(sim_plan))
    for i in range(len(sim_plan)):
        print("%.3d: %s" % (i, repr(sim_plan[i])))
    
    print("")

    for step in sim_plan:
        step.execute()


if __name__ == "__main__":
    main()
